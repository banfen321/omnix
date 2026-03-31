package validator

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var missingAttrRegex = regexp.MustCompile(`error: attribute '([^']+)' missing`)

type ValidationResult struct {
	Output       string
	MissingAttrs []string
	Success      bool
}

func Validate(nixDir string) *ValidationResult {
	// Use nix eval to evaluate the devShell directly — this reliably catches attribute errors
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes",
		"eval", nixDir+"#devShells.x86_64-linux.default", "--json")
	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))

	if err == nil {
		return &ValidationResult{Output: outStr, Success: true}
	}

	// Parse all missing attributes from the error output
	var missing []string
	matches := missingAttrRegex.FindAllStringSubmatch(outStr, -1)
	for _, m := range matches {
		if len(m) > 1 {
			missing = append(missing, m[1])
		}
	}

	return &ValidationResult{
		Output:       outStr,
		MissingAttrs: missing,
		Success:      false,
	}
}

// Check is kept for backward compatibility but uses Validate internally
func Check(nixDir string) (string, error) {
	result := Validate(nixDir)
	if result.Success {
		return result.Output, nil
	}
	return result.Output, fmt.Errorf("check failed")
}

func Eval(nixDir string) error {
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "eval", nixDir+"#devShells.x86_64-linux.default", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eval error: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
