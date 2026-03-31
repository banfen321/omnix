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
	failedRepairs := make(map[string]bool) // track attrs that were already repaired once

	for i := 0; i < 50; i++ {
		result := validator.Validate(nixDir)
		if result.Success {
			s.Stop()
			green.Println("  ✓ Validation passed")
			break
		}

		// FAST PATH: smart re-resolve for missing attributes
		if len(result.MissingAttrs) > 0 {
			s.Suffix = " Re-resolving broken packages..."
			for _, badAttr := range result.MissingAttrs {
				// If we already tried to repair this attr and it failed again → skip it
				if failedRepairs[badAttr] {
					var kept []resolver.NixPackage
					for _, p := range nixPkgs {
						if p.NixAttr == badAttr || strings.HasSuffix(p.NixAttr, "."+badAttr) {
							continue
						}
						kept = append(kept, p)
					}
					nixPkgs = kept
					yellow.Printf("  ⚠ Skipped '%s' (repair failed, will use language PM)\n", badAttr)
					continue
				}

				// Find the ecosystem of this broken package
				ecosystem := "unknown"
				for _, p := range nixPkgs {
					if p.NixAttr == badAttr || strings.HasSuffix(p.NixAttr, "."+badAttr) {
						ecosystem = p.Ecosystem
						break
					}
				}

				// Ask fast model: correct attr or SKIP?
				repair, repairErr := res.RepairPackage(badAttr, ecosystem, result.Output)
				repair = strings.TrimSpace(repair)

				if repairErr != nil || strings.EqualFold(repair, "SKIP") || repair == "" {
					// SKIP: remove from package list (pip/npm/cargo will handle it)
					var kept []resolver.NixPackage
					for _, p := range nixPkgs {
						if p.NixAttr == badAttr || strings.HasSuffix(p.NixAttr, "."+badAttr) {
							continue
						}
						kept = append(kept, p)
					}
					nixPkgs = kept
					yellow.Printf("  ⚠ Skipped '%s' (will be installed via language package manager)\n", badAttr)
				} else {
					// REPAIR: swap in the corrected attribute and mark as attempted
					for j, p := range nixPkgs {
						if p.NixAttr == badAttr || strings.HasSuffix(p.NixAttr, "."+badAttr) {
							nixPkgs[j].NixAttr = repair
							green.Printf("  ✓ Repaired '%s' → '%s'\n", badAttr, repair)
							break
						}
					}
					// Track the NEW attr so if it fails too, we skip next time
					failedRepairs[repair] = true
					// Also track the leaf name — nix reports just the leaf (e.g. 'clearml' not 'python3Packages.clearml')
					if idx := strings.LastIndex(repair, "."); idx >= 0 {
						failedRepairs[repair[idx+1:]] = true
					}
				}
			}
			// Regenerate flake from template with the updated package list
			if err := generator.New(info, nixPkgs).Generate(nixDir); err != nil {
				s.Stop()
				return fmt.Errorf("regenerate error: %w", err)
			}
			s.Suffix = " Validating flake..."
			continue
		}

		// SLOW PATH: LLM fallback for errors we can't parse with regex
		s.Suffix = " Fixing flake errors via LLM..."
		flakePath := filepath.Join(nixDir, "flake.nix")
		flakeContent, errRead := os.ReadFile(flakePath)
		if errRead != nil {
			s.Stop()
			return fmt.Errorf("read flake: %w", errRead)
		}

		fixed, fixErr := res.FixFlake(string(flakeContent), result.Output)
		if fixErr != nil {
			s.Stop()
			yellow.Printf("  ⚠ Validation warning: %s\n", result.Output)
			dim.Println("    flake.nix was generated but may need manual adjustments")
			break
		}

		if err := os.WriteFile(flakePath, []byte(fixed), 0o644); err != nil {
			s.Stop()
			return fmt.Errorf("write fixed flake: %w", err)
		}
		yellow.Println("  ⚠ LLM auto-fixed flake error. Retrying...")
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
