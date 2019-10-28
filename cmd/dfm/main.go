package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	dfm         Dfm = NewDfm()
	cliOptions  configFile
	addToRepo   string
	addWithCopy bool
)

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
	failed := false
	err := method(func(err FileError) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err)
			failed = true
		}
		return nil
	})
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
		fatal(fmt.Errorf("no repos are configured and no repo was specifed"))
		return
	}
	failed := false
	err := dfm.AddFiles(args, addToRepo, !addWithCopy, func(err FileError) error {
		if os.IsNotExist(err.Cause()) {
			fmt.Fprintf(os.Stderr, "%s: no such file or directory\n", err.Filename())
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
		failed = true
		return nil
	})

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
	if dfm.Config.path == "" {
		var exists bool
		if dfm.Config.path, exists = os.LookupEnv("DFM_DIR"); !exists {
			var err error
			if dfm.Config.path, err = os.Getwd(); err != nil {
				panic(err)
			}
		}
	}
	if err := dfm.Config.SetDirectory(dfm.Config.path); err != nil {
		fatal(err)
		return
	}
	dfm.Config.applyFile(cliOptions)
}

func main() {
	cobra.OnInitialize(initConfig)

	var rootCmd = &cobra.Command{
		Use:     "dfm",
		Version: "1.0.0",
		Long:    "Manages your dotfiles",
	}
	rootCmd.PersistentFlags().StringVarP(&dfm.Config.path, "dfm-dir", "d", "", "directory where dfm repositories live")
	rootCmd.PersistentFlags().StringArrayVarP(&cliOptions.Repos, "repos", "R", nil, "repositories to track")
	rootCmd.PersistentFlags().StringVar(&dfm.Config.targetPath, "target", dfm.Config.targetPath, "directory to sync files in")

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
