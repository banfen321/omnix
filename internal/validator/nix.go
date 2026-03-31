package validator

import (
	"fmt"
	"os/exec"
	"strings"
)

func Check(nixDir string) error {
	cmd := exec.Command("nix", "flake", "check", nixDir, "--no-build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

func Eval(nixDir string) error {
	cmd := exec.Command("nix", "eval", nixDir+"#devShells.x86_64-linux.default", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("eval error: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
