package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	config      DfmConfig = NewDfmConfig()
	cliOptions  configFile
	addToRepo   string
	addWithCopy bool
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err.Error())
	os.Exit(1)
}

func runInit(cmd *cobra.Command, args []string) {
	err := config.Save()
	if err != nil {
		fatal(err)
		return
	}
	fmt.Printf("Initialized %s as a dfm directory.\n", config.path)
}

func runSync(handleFile func(s, d string) error) {
	fileList := map[string]string{}
	for _, repo := range config.repos {
		repoPath := config.RepoPath(repo, "")
		filepath.Walk(repoPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				fatal(err)
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			relativePath := path[len(repoPath)+1:]
			fileList[relativePath] = repo
			return nil
		})
	}
	failed := false
	newManifest := make(map[string]bool, len(fileList))
	for relative, repo := range fileList {
		repoPath := config.RepoPath(repo, relative)
		targetPath := config.TargetPath(relative)
		if err := handleFile(repoPath, targetPath); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			failed = true
			continue
		}
		newManifest[relative] = true
	}
	// XXX run auto clean
	config.manifest = newManifest
	config.Save()
	if failed {
		os.Exit(1)
	}
}

func runLink(cmd *cobra.Command, args []string) {
	runSync(func(s, d string) error {
		// XXX - check if link is already correct
		return LinkFile(s, d)
	})
}

func runCopy(cmd *cobra.Command, args []string) {
	runSync(func(s, d string) error {
		// XXX - check if files are equivalent
		return CopyFile(s, d)
	})
}

func addSingleFile(filename string) error {
	targetPath, err := filepath.Abs(filename)
	if err != nil {
		fatal(err)
		return nil
	}
	// Verify file is under targetPath
	if !strings.HasPrefix(targetPath, config.targetPath+"/") {
		return fmt.Errorf("not in target path (%s)", config.targetPath)
	}
	relativePath := targetPath[len(config.targetPath)+1:]
	stat, err := os.Lstat(targetPath)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return fmt.Errorf("directories are not supported")
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("only regular files are supported")
	}
	repoPath := config.RepoPath(addToRepo, relativePath)
	if err := MakeDirAll(path.Dir(relativePath), config.targetPath, config.RepoPath(addToRepo, "")); err != nil {
		return err
	}
	if addWithCopy {
		if err := CopyFile(targetPath, repoPath); err != nil {
			return err
		}
	} else {
		if err := MoveFile(targetPath, repoPath); err != nil {
			return err
		}
		if err := LinkFile(repoPath, targetPath); err != nil {
			return err
		}
	}
	config.AddToManifest(addToRepo, relativePath)
	return nil
}

// Copy the given files into the repository and replace them with symlinks
func runAdd(cmd *cobra.Command, args []string) {
	// If there is only one repo, allow add without specifying which one.
	if addToRepo == "" && len(config.repos) == 1 {
		addToRepo = config.repos[0]
	}
	if addToRepo == "" {
		fatal(fmt.Errorf("no repos are configured and no repo was specifed"))
		return
	} else if !config.IsValidRepo(addToRepo) {
		fatal(fmt.Errorf("repo %#v does not exist. To create it, run:\nmkdir %s", addToRepo, config.RepoPath(addToRepo, "")))
		return
	} else if !config.HasRepo(addToRepo) {
		fatal(fmt.Errorf("repo %#v is not active, cannot add files to it", addToRepo))
		return
	}

	failed := false
	for _, filename := range args {
		if err := addSingleFile(filename); err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "%s: no such file or directory\n", filename)
			} else {
				fmt.Fprintf(os.Stderr, "%s: %s\n", filename, err.Error())
			}
			failed = true
		}
	}
	err := config.Save()
	if err != nil {
		fatal(err)
		return
	}
	if failed {
		os.Exit(1)
	}
}

func runRemove(cmd *cobra.Command, args []string) {
	// XXX just run an auto clean with an empty manifest
	fmt.Printf("removing...\n")
}

func initConfig() {
	if config.path == "" {
		var exists bool
		if config.path, exists = os.LookupEnv("DFM_DIR"); !exists {
			var err error
			if config.path, err = os.Getwd(); err != nil {
				panic(err)
			}
		}
	}
	if err := config.SetDirectory(config.path); err != nil {
		fatal(err)
		return
	}
	config.applyFile(cliOptions)
	fmt.Printf("%+v\n", config)
}

func main() {
	cobra.OnInitialize(initConfig)

	var rootCmd = &cobra.Command{
		Use:     "dfm",
		Version: "1.0.0",
		Long:    "Manages your dotfiles",
	}
	rootCmd.PersistentFlags().StringVarP(&config.path, "dfm-dir", "d", "", "directory where dfm repositories live")
	rootCmd.PersistentFlags().StringArrayVarP(&cliOptions.Repos, "repos", "R", nil, "repositories to track")
	rootCmd.PersistentFlags().StringVar(&config.targetPath, "target", config.targetPath, "directory to sync files in")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize the dfm directory",
		Long:  "Create a dfm.toml file in the dfm directory with the default configuration. Use --dfm-dir to specify the dfm directory.",
		Args:  cobra.NoArgs,
		Run:   runInit,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "link",
		Short: "Create symlinks to tracked files",
		Args:  cobra.NoArgs,
		Run:   runLink,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "copy",
		Short: "Create copies of tracked files",
		Args:  cobra.NoArgs,
		Run:   runCopy,
	})

	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Begin tracking files",
		Long:  "Copy the given files into the repository and replace the originals with links to the tracked files.",
		Args:  cobra.MinimumNArgs(1),
		Run:   runAdd,
	}
	addCmd.Flags().StringVarP(&addToRepo, "repo", "r", "", "repository to add the file to")
	addCmd.Flags().BoolVar(&addWithCopy, "copy", false, "copy the file instead of moving and creating a link")
	rootCmd.AddCommand(addCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove all synced files",
		Args:    cobra.NoArgs,
		Run:     runRemove,
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
