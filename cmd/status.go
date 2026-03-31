package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/banfen321/OmniNix/internal/config"
	"github.com/banfen321/OmniNix/internal/storage"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current environment status",
	Long:  "Display detected stack, resolved packages, and environment state",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	nixDir := filepath.Join(dir, ".nix")
	flakeFile := filepath.Join(nixDir, "flake.nix")
	envrcFile := filepath.Join(dir, ".envrc")

	cyan.Printf("  omnix status for %s\n\n", filepath.Base(dir))

	hasFlake := false
	hasEnvrc := false

	if _, err := os.Stat(flakeFile); err == nil {
		hasFlake = true
		green.Println("  ✓ .nix/flake.nix exists")
	} else {
		red.Println("  ✗ .nix/flake.nix not found")
	}

	if _, err := os.Stat(envrcFile); err == nil {
		hasEnvrc = true
		green.Println("  ✓ .envrc exists")
	} else {
		red.Println("  ✗ .envrc not found")
	}

	if direnvActive := os.Getenv("DIRENV_DIR"); direnvActive != "" {
		green.Println("  ✓ direnv active")
	} else {
		if hasFlake && hasEnvrc {
			yellow.Println("  ⚠ direnv not active (run 'direnv allow')")
		} else {
			dim.Println("  - direnv not active")
		}
	}

	if hasFlake {
		fmt.Println()
		data, err := os.ReadFile(flakeFile)
		if err == nil {
			content := string(data)
			if idx := strings.Index(content, "buildInputs"); idx != -1 {
				dim.Println("  Packages in flake:")
				lines := strings.Split(content[idx:], "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed == "];" {
						break
					}
					if trimmed != "" && trimmed != "buildInputs = [" && !strings.HasPrefix(trimmed, "buildInputs") {
						dim.Printf("    %s\n", trimmed)
					}
				}
			}
		}
	}

	cfg, err := config.Load()
	if err == nil {
		db, err := storage.OpenSQLite(cfg.SQLitePath)
		if err == nil {
			defer db.Close()
			hash, _ := hashProject(dir)
			if cached, err := db.GetCache(hash); err == nil && cached != "" {
				fmt.Println()
				green.Println("  ✓ Cache valid")
				dim.Printf("    hash: %s\n", hash[:12])
			}
		}
	}

	return nil
}
