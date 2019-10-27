package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var config DfmConfig = NewDfmConfig()
var cliOptions configFile

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err.Error())
	os.Exit(1)
}

func runInit(cmd *cobra.Command, args []string) {
	err := config.SaveConfig()
	if err != nil {
		fatal(err)
		return
	}
	fmt.Printf("Initialized %s as a dfm directory.\n", config.path)
}

func runLink(cmd *cobra.Command, args []string) {
	fmt.Printf("linking...\n")
}

func runCopy(cmd *cobra.Command, args []string) {
	fmt.Printf("copying...\n")
}

func runAdd(cmd *cobra.Command, args []string) {
	fmt.Printf("adding... %v\n", args)
}

func runRemove(cmd *cobra.Command, args []string) {
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
}

func main() {
	cobra.OnInitialize(initConfig)

	var rootCmd = &cobra.Command{
		Use:     "dfm",
		Version: "1.0.0",
		Long:    "Manages your dotfiles",
	}
	rootCmd.PersistentFlags().StringVarP(&config.path, "dfm-dir", "d", "", "Directory where dfm repositories live")
	rootCmd.PersistentFlags().StringArrayVarP(&cliOptions.Repos, "repos", "r", nil, "Repositories to track")

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

	rootCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Begin tracking a file",
		Long:  "Copy the given files into the repository and replace the originals with links to the tracked files.",
		Args:  cobra.MinimumNArgs(1),
		Run:   runAdd,
	})

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
