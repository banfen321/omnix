package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Short:   "Show omnix version",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}

func init() {
	rootCmd.Version = fmt.Sprintf("%s (%s)", Version, Commit)
	rootCmd.SetVersionTemplate("omnix v{{.Version}}\n")
}

func printVersion() {
	fmt.Printf("omnix v%s (%s)\n", Version, Commit)
	os.Exit(0)
}
