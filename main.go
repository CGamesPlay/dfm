package main

import (
	"fmt"
	"os"
	"path/filepath"

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

func runInit(cmd *cobra.Command, args []string) {
	if err := dfm.Init(); err != nil {
		fatal(err)
		return
	}
	fmt.Printf("Initialized %s as a dfm directory.\n", dfm.Config.path)
}

func runSync(method func(errorHandler ErrorHandler) error) {
	err := method(errorHandler)
	if err != nil {
		fatal(err)
		return
	}
	if failed {
		os.Exit(1)
	}
}

func runLink(cmd *cobra.Command, args []string) {
	runSync(dfm.LinkAll)
}

func runCopy(cmd *cobra.Command, args []string) {
	runSync(dfm.CopyAll)
}

// Copy the given files into the repository and replace them with symlinks
func runAdd(cmd *cobra.Command, args []string) {
	// If there is only one repo, allow add without specifying which one.
	if addToRepo == "" && len(dfm.Config.repos) == 1 {
		addToRepo = dfm.Config.repos[0]
	}
	if addToRepo == "" {
		fatal(fmt.Errorf("no repos are configured and no repo was specified"))
		return
	}
	err := dfm.AddFiles(args, addToRepo, !addWithCopy, errorHandler)

	if err != nil {
		fatal(err)
		return
	}
	if failed {
		os.Exit(1)
	}
}

func runRemove(cmd *cobra.Command, args []string) {
	dfm.RemoveAll()
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
	initCmd.Flags().StringArrayVar(&cliOptions.Repos, "repos", nil, "repositories to track")
	initCmd.Flags().StringVar(&cliOptions.Target, "target", "", "directory to place files in")
	rootCmd.AddCommand(initCmd)

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
		Use:     "add [files]",
		Aliases: []string{"import"},
		Short:   "Begin tracking files",
		Long: wordwrap.WrapString(`Copy the given files into the repository and replace the originals with links to the tracked files.

If no repo is specified in the command, the repo that is listed last will be used.`, 80),
		Args: cobra.MinimumNArgs(1),
		Run:  runAdd,
	}
	addCmd.Flags().StringVarP(&addToRepo, "repo", "r", "", "repository to add the file to")
	addCmd.Flags().BoolVar(&addWithCopy, "copy", false, "copy the file instead of moving and creating a link")
	rootCmd.AddCommand(addCmd)

	rootCmd.AddCommand(&cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove all tracked files",
		Args:    cobra.NoArgs,
		Run:     runRemove,
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
