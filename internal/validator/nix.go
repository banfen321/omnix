package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	missingAttrRegex = regexp.MustCompile(`error: attribute '([^']+)' missing`)
	renamedAttrRegex = regexp.MustCompile(`error: '([^']+)' has been renamed to/replaced by '([^']+)'`)
)

type RenamedAttr struct {
	Old string
	New string
}

type ValidationResult struct {
	Output       string
	MissingAttrs []string
	RenamedAttrs []RenamedAttr
	Success      bool
}

// LockFlake locks the flake inputs once — call before the repair loop
func LockFlake(nixDir string) {
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes",
		"flake", "lock", nixDir)
	_ = cmd.Run()

	// If in a git repo, we MUST add the generated lock file to git.
	// Otherwise nix eval --no-update-lock-file will fail because it ignores untracked files in git repositories
	if _, err := os.Stat(filepath.Join(filepath.Dir(nixDir), ".git")); err == nil {
		_ = exec.Command("git", "-C", filepath.Dir(nixDir), "add", "-f", ".nix/flake.lock").Run()
	}
}

// Validate evaluates the devShell with --no-update-lock-file (fast after LockFlake)
func Validate(nixDir string) *ValidationResult {
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes",
		"eval", nixDir+"#devShells.x86_64-linux.default", "--json", "--no-update-lock-file")
	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))

	if err == nil {
		return &ValidationResult{Output: outStr, Success: true}
	}

	// Parse missing attributes
	var missing []string
	matches := missingAttrRegex.FindAllStringSubmatch(outStr, -1)
	for _, m := range matches {
		if len(m) > 1 {
			missing = append(missing, m[1])
		}
	}

	// Parse renamed attributes
	var renamed []RenamedAttr
	rmatches := renamedAttrRegex.FindAllStringSubmatch(outStr, -1)
	for _, m := range rmatches {
		if len(m) > 2 {
			renamed = append(renamed, RenamedAttr{Old: m[1], New: m[2]})
		}
	}

	return &ValidationResult{
		Output:       outStr,
		MissingAttrs: missing,
		RenamedAttrs: renamed,
		Success:      false,
	}
}
