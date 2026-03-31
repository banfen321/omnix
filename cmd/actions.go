package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Rescan project and update Nix environment",
	Long:  "Re-analyze project dependencies and regenerate .nix/ directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		scanForce = true
		return runScan(cmd, args)
	},
}

var rmCmd = &cobra.Command{
	Use:     "rm",
	Aliases: []string{"remove"},
	Short:   "Remove generated Nix environment",
	Long:    "Delete .nix/ directory, .envrc, and clean .gitignore entries",
	RunE:    runRm,
}

func runRm(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	nixDir := dir + "/.nix"
	envrcFile := dir + "/.envrc"

	removed := false

	if _, err := os.Stat(nixDir); err == nil {
		if err := os.RemoveAll(nixDir); err != nil {
			return fmt.Errorf("cannot remove .nix/: %w", err)
		}
		green.Println("  ✓ Removed .nix/")
		removed = true
	}

	if _, err := os.Stat(envrcFile); err == nil {
		data, err := os.ReadFile(envrcFile)
		if err == nil {
			content := string(data)
			if content == "use flake ./.nix\n" || content == "use flake ./.nix" {
				if err := os.Remove(envrcFile); err != nil {
					return fmt.Errorf("cannot remove .envrc: %w", err)
				}
				green.Println("  ✓ Removed .envrc")
				removed = true
			} else {
				yellow.Println("  ⚠ .envrc has custom content, skipping")
			}
		}
	}

	if !removed {
		dim.Println("  Nothing to remove")
	}

	return nil
}
