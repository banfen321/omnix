package validator

import (
	"fmt"
	"os/exec"
	"strings"
)

func Check(nixDir string) (string, error) {
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "flake", "check", nixDir, "--no-build")
	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))
	if err != nil {
		if outStr == "" {
			outStr = err.Error()
		}
		return outStr, fmt.Errorf("check failed")
	}
	return outStr, nil
}

func Eval(nixDir string) error {
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "eval", nixDir+"#devShells.x86_64-linux.default", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eval error: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
