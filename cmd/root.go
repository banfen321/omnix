package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	Version = "0.5.3"
	Commit  = "production-ready"
)

var (
	bold   = color.New(color.Bold)
	cyan   = color.New(color.FgCyan, color.Bold)
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	red    = color.New(color.FgRed)
	dim    = color.New(color.Faint)
)

var rootCmd = &cobra.Command{
	Use:   "omnix",
	Short: "AI-powered Nix dev environment generator",
	Long: `  omnix — AI-powered Nix dev environment generator

  Scan your project, resolve dependencies using AST parsing + LLM,
  and generate a ready-to-use Nix flake with direnv integration.

  Quick Start:
    $ omnix conf          # configure API keys & settings
    $ omnix sync          # index nixpkgs into local SQLite
    $ omnix scan          # scan project, generate .nix/ & auto-activate
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		red.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.SetHelpTemplate(helpTemplate())
	rootCmd.SetUsageTemplate(usageTemplate())

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(confCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(versionCmd)
}

func helpTemplate() string {
	return fmt.Sprintf(`%s

%s
{{if .HasAvailableSubCommands}}
%s{{range .Commands}}{{if .IsAvailableCommand}}
  %s  {{.Short}}{{end}}{{end}}
{{end}}
%s
  {{.CommandPath}} [command] --help

%s
  %s
  %s
`,
		`{{.Long}}`,
		bold.Sprint("Usage:"),
		bold.Sprint("Commands:"),
		`{{rpad .Name .NamePadding}}`,
		bold.Sprint("Help:"),
		bold.Sprint("Version:"),
		fmt.Sprintf("omnix v%s (%s)", Version, Commit),
		dim.Sprint("https://github.com/banfen321/omnix"),
	)
}

func usageTemplate() string {
	return `Usage:
  {{.UseLine}}{{if .HasAvailableSubCommands}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}

Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`
}
