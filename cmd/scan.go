package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/banfen321/omnix/internal/config"
	"github.com/banfen321/omnix/internal/generator"
	"github.com/banfen321/omnix/internal/resolver"
	"github.com/banfen321/omnix/internal/scanner"
	"github.com/banfen321/omnix/internal/storage"
	"github.com/banfen321/omnix/internal/validator"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

var (
	scanDryRun      bool
	scanForce       bool
	scanNoGitignore bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan project and generate Nix environment",
	Long:  "Analyze project files, resolve dependencies via RAG + LLM, generate .nix/ directory with flake.nix and .envrc",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().BoolVar(&scanDryRun, "dry-run", false, "show what would be generated without writing files")
	scanCmd.Flags().BoolVarP(&scanForce, "force", "f", false, "regenerate even if cache is valid")
	scanCmd.Flags().BoolVar(&scanNoGitignore, "no-gitignore", false, "skip .gitignore management")
}

func runScan(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config error: %w (run 'omnix conf' first)", err)
	}

	cyan.Printf("  Scanning %s\n", dir)

	projectHash, err := hashProject(dir)
	if err != nil {
		return fmt.Errorf("hash error: %w", err)
	}

	db, err := storage.OpenSQLite(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("storage error: %w", err)
	}
	defer db.Close()

	if !scanForce {
		cached, err := db.GetCache(projectHash)
		if err == nil && cached != "" {
			// Also verify the physical files haven't been deleted
			flakeExists := false
			envrcExists := false
			if _, err := os.Stat(filepath.Join(dir, ".nix", "flake.nix")); err == nil {
				flakeExists = true
			}
			if _, err := os.Stat(filepath.Join(dir, ".envrc")); err == nil {
				envrcExists = true
			}

			if flakeExists && envrcExists {
				green.Println("  ✓ Using cached environment (unchanged)")
				dim.Printf("    hash: %s\n", projectHash[:12])
				return nil
			}
		}
	}

	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Prefix = "  "

	s.Suffix = " Detecting project stack..."
	s.Start()

	det := scanner.NewDetector()
	info, err := det.Scan(dir)
	if err != nil {
		s.Stop()
		return fmt.Errorf("scan error: %w", err)
	}

	if len(info.Dependencies) == 0 {
		s.Stop()
		yellow.Println("  ⚠ No dependencies detected")
		return nil
	}

	s.Suffix = fmt.Sprintf(" Found %d dependencies across [%s]", len(info.Dependencies), strings.Join(info.Stacks, ", "))
	time.Sleep(500 * time.Millisecond)

	s.Suffix = " Resolving nix packages..."

	res, err := resolver.New(cfg, db)
	if err != nil {
		s.Stop()
		return fmt.Errorf("resolver error: %w", err)
	}

	nixPkgs, err := res.Resolve(info.Dependencies)
	if err != nil {
		s.Stop()
		return fmt.Errorf("resolve error: %w", err)
	}

	s.Stop()

	var resolved []resolver.NixPackage
	var unresolved []resolver.NixPackage

	for _, p := range nixPkgs {
		if p.Source == "fallback" || strings.Contains(p.NixAttr, "/") || strings.Contains(p.NixAttr, " ") {
			unresolved = append(unresolved, p)
		} else {
			resolved = append(resolved, p)
		}
	}

	green.Printf("  ✓ Resolved %d packages\n", len(resolved))
	for _, p := range resolved {
		fmt.Printf("    %s → %s\n", p.OriginalName, p.NixAttr)
	}

	if len(unresolved) > 0 {
		fmt.Println()
		yellow.Printf("  ⚠ Could not resolve %d packages (these will NOT be added to your Nix environment):\n", len(unresolved))
		for _, p := range unresolved {
			fmt.Printf("    %s\n", p.OriginalName)
		}
		dim.Println("    (You can install them manually or add them to static mappings later)")
	}

	if scanDryRun {
		yellow.Println("\n  [dry-run] No files written")
		return nil
	}

	s = spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Prefix = "  "
	s.Suffix = " Generating .nix/ directory..."
	s.Start()

	gen := generator.New(info, nixPkgs)
	nixDir := filepath.Join(dir, ".nix")

	if err := gen.Generate(nixDir); err != nil {
		s.Stop()
		return fmt.Errorf("generate error: %w", err)
	}

	if err := gen.GenerateEnvrc(dir); err != nil {
		s.Stop()
		return fmt.Errorf("envrc error: %w", err)
	}

	if !scanNoGitignore && cfg.AutoGitignore {
		if err := generator.PatchGitignore(dir); err != nil {
			s.Stop()
			yellow.Printf("  ⚠ gitignore: %s\n", err)
		}
	}

	s.Suffix = " Validating flake..."

	if err := validator.Check(nixDir); err != nil {
		s.Stop()
		yellow.Printf("  ⚠ Validation warning: %s\n", err)
		dim.Println("    flake.nix was generated but may need manual adjustments")
	} else {
		s.Stop()
		green.Println("  ✓ Validation passed")
	}

	if err := db.PutCache(projectHash, nixDir); err != nil {
		dim.Printf("  cache warning: %s\n", err)
	}

	fmt.Println()
	bold.Println("  Activating via direnv...")

	cmdDirenv := exec.Command("direnv", "allow")
	cmdDirenv.Dir = dir
	cmdDirenv.Stdout = os.Stdout
	cmdDirenv.Stderr = os.Stderr
	if err := cmdDirenv.Run(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			fmt.Println()
			yellow.Println("  ⚠ 'direnv' is not installed in your system!")
			dim.Println("    direnv is required for MAGIC auto-activation when you enter the folder.")

			bold.Println("  ⏳ Automatically installing direnv via nix profile...")
			cmdInst := exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "profile", "install", "nixpkgs#direnv", "nixpkgs#nix-direnv")
			cmdInst.Stdout = os.Stdout
			cmdInst.Stderr = os.Stderr
			if err := cmdInst.Run(); err != nil {
				yellow.Printf("\n  ⚠ Failed to install direnv: %s\n", err)
			} else {
				green.Println("\n  ✓ direnv installed successfully!")
				hookDirenvToRC()
				dim.Println("    Please RESTART your terminal or run this command ONCE to activate it now:")
				bold.Println("      eval \"$(direnv hook $(basename $SHELL))\"")

				// try again just in case
				exec.Command("direnv", "allow").Run()
			}

			fmt.Println()
			green.Println("  ✓ But your Nix environment is built!")
			bold.Println("  ▶ To enter it manually right now, run: nix --extra-experimental-features \"nix-command flakes\" develop ./.nix")
		} else {
			yellow.Printf("  ⚠ Could not auto-run 'direnv allow': %s\n", err)
		}
	} else {
		dim.Println("  ✓ 'direnv allow' executed automatically.")
	}

	return nil
}

func hashProject(dir string) (string, error) {
	manifestFiles := []string{
		"requirements.txt", "pyproject.toml", "setup.py", "setup.cfg", "Pipfile",
		"package.json", "package-lock.json", "yarn.lock",
		"go.mod", "go.sum",
		"Cargo.toml", "Cargo.lock",
		"Gemfile", "Gemfile.lock",
		"composer.json", "composer.lock",
		"build.gradle", "pom.xml",
		"Dockerfile", "docker-compose.yml",
		"Makefile", ".tool-versions",
		"*.tf",
	}

	h := sha256.New()
	var found []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == ".nix" || base == ".direnv" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		for _, pattern := range manifestFiles {
			matched, _ := filepath.Match(pattern, name)
			if matched {
				found = append(found, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(found)
	for _, f := range found {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		h.Write([]byte(f))
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func hookDirenvToRC() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	shell := os.Getenv("SHELL")
	rcFile := filepath.Join(home, ".bashrc")
	hookCmd := `eval "$(direnv hook bash)"`

	if strings.Contains(shell, "zsh") {
		rcFile = filepath.Join(home, ".zshrc")
		hookCmd = `eval "$(direnv hook zsh)"`
	}

	content, err := os.ReadFile(rcFile)
	if err == nil && strings.Contains(string(content), "direnv hook") {
		return
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString("\n# Added by omnix\n" + hookCmd + "\n")
		dim.Printf("    ✓ Successfully added direnv hook to %s (Restart terminal to apply)\n", rcFile)
	}
}
