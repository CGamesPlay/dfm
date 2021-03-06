package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	"github.com/spf13/cobra"
)

var (
	dfmDir      string
	dfm         *Dfm
	cliOptions  configFile
	verbose     bool
	dryRun      bool
	force       bool
	addToRepo   string
	addWithCopy bool
	failed      bool
)

func defaultLogger(operation, relative, repo string, reason error) {
	switch operation {
	case OperationLink, OperationCopy:
		fmt.Printf("%s -> %s\n", pathJoin(repo, relative), dfm.TargetPath(relative))
	case OperationSkip:
		if IsNotNeeded(reason) && !verbose {
			return
		} else if fileErr, ok := reason.(*FileError); ok {
			reason = fmt.Errorf(fileErr.Message)
		}
		fmt.Printf("skipping %s: %s\n", dfm.TargetPath(relative), reason)
	default:
		fmt.Printf("%s %s\n", operation, relative)
	}
}

func errorHandler(fileError *FileError) error {
	if force && os.IsExist(fileError.Cause()) {
		var removeErr error
		if linkErr, ok := fileError.Cause().(*os.LinkError); ok {
			removeErr = os.Remove(linkErr.New)
		} else if pathErr, ok := fileError.Cause().(*os.PathError); ok {
			removeErr = os.Remove(pathErr.Path)
		} else {
			removeErr = fileError.Cause()
		}
		if removeErr != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", fileError.Filename, removeErr)
			return nil
		}
		return Retry
	}
	failed = true
	return nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err.Error())
	os.Exit(1)
}

func handleCommandError(err error) {
	if err != nil {
		fatal(err)
		return
	}
	if failed {
		os.Exit(2)
	}
}

// resolveInputFilenames transforms the given list of filenames to relative
// paths in the target directory, taking into account the pwd. Errors will
// abort the program.
func resolveInputFilenames(filenames []string, allowRepoPath bool) []string {
	targetPath := dfm.TargetPath("")
	allowedPrefixes := make([]string, 0, len(dfm.Config.repos)+1)
	if allowRepoPath {
		for _, repo := range dfm.Config.repos {
			allowedPrefixes = append(allowedPrefixes, dfm.RepoPath(repo, ""))
		}
	}
	allowedPrefixes = append(allowedPrefixes, targetPath)

	results := make([]string, 0, len(filenames))
	for _, input := range filenames {
		absolute, err := filepath.Abs(input)
		if err != nil {
			// If Abs fails, none of the paths will be valid. Just abort.
			fatal(err)
		}
		found := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(absolute, prefix) {
				results = append(results, absolute[len(prefix)+1:])
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "%s: not in target path (%s)\n", input, targetPath)
			failed = true
		}
	}
	if failed {
		os.Exit(2)
	}
	return results
}

func runInit(cmd *cobra.Command, args []string) {
	handleCommandError(dfm.Init())
	fmt.Printf("Initialized %s as a dfm directory.\n", dfm.Config.path)
}

func runLink(cmd *cobra.Command, args []string) {
	var err error
	if len(args) == 0 {
		err = dfm.LinkAll(errorHandler)
	} else {
		err = dfm.LinkFiles(resolveInputFilenames(args, true), errorHandler)
	}
	handleCommandError(err)
}

func runCopy(cmd *cobra.Command, args []string) {
	var err error
	if len(args) == 0 {
		err = dfm.CopyAll(errorHandler)
	} else {
		err = dfm.CopyFiles(resolveInputFilenames(args, true), errorHandler)
	}
	handleCommandError(err)
}

// Copy the given files into the repository and replace them with symlinks
func runAdd(cmd *cobra.Command, args []string) {
	// If there is only one repo, allow add without specifying which one.
	if addToRepo == "" {
		if len(dfm.Config.repos) == 0 {
			fatal(fmt.Errorf("no repos are configured. Have you run dfm init?"))
			return
		} else if len(dfm.Config.repos) > 1 {
			fatal(fmt.Errorf("repo must be specified when multiple are configured"))
			return
		} else {
			addToRepo = dfm.Config.repos[0]
		}
	}
	err := dfm.AddFiles(resolveInputFilenames(args, false), addToRepo, !addWithCopy, errorHandler)
	handleCommandError(err)
}

func runRemove(cmd *cobra.Command, args []string) {
	var err error
	if len(args) == 0 {
		err = dfm.RemoveAll()
	} else {
		err = dfm.RemoveFiles(resolveInputFilenames(args, true))
	}
	handleCommandError(err)
}

func runEject(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{"."}
	} else {
		args = resolveInputFilenames(args, false)
	}
	handleCommandError(dfm.EjectFiles(args, errorHandler))
}

func initConfig() {
	var err error
	if dfmDir == "" {
		var exists bool
		if dfmDir, exists = os.LookupEnv("DFM_DIR"); !exists {
			if dfmDir, err = os.Getwd(); err != nil {
				panic(err)
			}
		}
	}
	dfm, err = NewDfm(dfmDir)
	if err != nil {
		fatal(err)
		return
	}
	dfm.DryRun = dryRun
	dfm.Logger = defaultLogger
	if cliOptions.Target != "" {
		absPath, err := filepath.Abs(cliOptions.Target)
		if err != nil {
			fatal(err)
			return
		}
		cliOptions.Target = absPath
	}
	dfm.Config.applyFile(cliOptions)
}

func main() {
	cobra.OnInitialize(initConfig)

	var rootCmd = &cobra.Command{
		Use:     "dfm",
		Version: Version,
		Long: wordwrap.WrapString(`dfm is a tool to manage repositories of configuration files. A simple workflow for dfm might look like this:

  mkdir -p ~/dotfiles/files; cd ~/dotfiles
  dfm init --repos files
  dfm add ~/.bashrc

Now ~/dotfiles can be tracked in source control, and to install on another machine you would use:

  cd ~/dotfiles
  dfm init --repos files
  dfm link

Note that .dfm.toml is a per-machine configuration and should not be tracked in source control.

`, 80),
	}
	rootCmd.PersistentFlags().StringVarP(&dfmDir, "dfm-dir", "d", "", "directory where dfm repositories live")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "output every file, even unchanged ones")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would happen, but don't actually modify files")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "overwrite files that already exist")

	rootCmd.SetUsageTemplate(rootCmd.UsageTemplate() + "\n" + CopyrightString + "\n")

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the dfm directory",
		Long: wordwrap.WrapString(`Initialize a directory to be used with dfm by creating the .dfm.toml file there.

Specifying --repos and --target will allow you to configure which repos are used and where the files should be stored. It is safe to run dfm init on an already-initialized dfm directory, to change the repos that are being used.`, 80),
		Example: `  dfm init --repos files`,
		Args:    cobra.NoArgs,
		Run:     runInit,
	}
	initCmd.Flags().StringSliceVar(&cliOptions.Repos, "repos", nil, "repositories to track")
	initCmd.Flags().StringVar(&cliOptions.Target, "target", "", "directory to place files in")
	rootCmd.AddCommand(initCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:   "link [files]",
		Short: "Create symlinks to tracked files",
		Args:  cobra.ArbitraryArgs,
		Run:   runLink,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "copy [files]",
		Short: "Create copies of tracked files",
		Args:  cobra.ArbitraryArgs,
		Run:   runCopy,
	})

	addCmd := &cobra.Command{
		Use:     "add [files]",
		Aliases: []string{"import"},
		Short:   "Begin tracking files",
		Long: wordwrap.WrapString(`Copy the given files into the repository and replace the originals with links to the tracked files.

This command is a convenient way to replace the following 2 commands:
  mv ~/myfile $DFM_DIR/files/myfile
  dfm link ~/myfile`, 80),
		Args: cobra.MinimumNArgs(1),
		Run:  runAdd,
	}
	addCmd.Flags().StringVarP(&addToRepo, "repo", "r", "", "repository to add the file to")
	addCmd.Flags().BoolVar(&addWithCopy, "copy", false, "copy the file instead of moving and creating a link")
	rootCmd.AddCommand(addCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:     "remove [files]",
		Aliases: []string{"rm"},
		Short:   "Remove tracked files",
		Long: wordwrap.WrapString(`Remove files from the target directory. The files will remain in the dfm repo, so they will be recreated the next time dfm copy or dfm link is run.

To remove a config file from a dfm repo entirely, simply delete the file and run dfm link or dfm copy. Then dfm will automatically clean up the deleted file.

This command is only useful if you want dfm to stop tracking a file, but dfm eject is a more convenient way of doing this.`, 80),
		Args: cobra.ArbitraryArgs,
		Run:  runRemove,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "eject [files]",
		Short: "Stop tracking files",
		Long: wordwrap.WrapString(`Copy the given files into the target directory without tracking them. This means that dfm link will refuse to overwrite the files (without --force), and removing the files will not cause the autoclean to remove them from the target directory.

This command is meant to be used when you want to keep a config file, but stop tracking it with dfm. Once you have ejected a file, it is safe to remove from the dfm repo. Note: if your dfm repo is shared between multiple machines, any other machines will NOT correctly eject the file: on other machines, it will appear as though the file has been deleted normally.

This command is the inverse of dfm add, and is a convenient way to replace the following 2 commands:
  dfm remove ~/myfile
  cp $DFM_DIR/files/myfile ~/myfile`, 80),
		Args: cobra.ArbitraryArgs,
		Run:  runEject,
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
