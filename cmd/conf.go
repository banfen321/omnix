package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/banfen321/OmniNix/internal/config"
	"github.com/spf13/cobra"
)

var confCmd = &cobra.Command{
	Use:     "conf",
	Aliases: []string{"config", "init"},
	Short:   "Configure omnix settings",
	Long:    "Interactive configuration for API keys, model selection, and preferences",
	RunE:    runConf,
}

var confSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a specific config value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfSet,
}

var confGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a specific config value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfGet,
}

var confShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfShow,
}

func init() {
	confCmd.AddCommand(confSetCmd)
	confCmd.AddCommand(confGetCmd)
	confCmd.AddCommand(confShowCmd)
}

func runConf(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Default()
	}

	reader := bufio.NewReader(os.Stdin)

	cyan.Println("  omnix configuration")
	fmt.Println()

	cfg.APIProvider = prompt(reader, "API provider (openrouter/google)", cfg.APIProvider)
	newKey := prompt(reader, "API key", maskKey(cfg.APIKey))
	if newKey != maskKey(cfg.APIKey) {
		cfg.APIKey = newKey
	}

	fmt.Println()
	bold.Println("  Models:")
	cfg.FastModel = prompt(reader, "  Fast model (analysis)", cfg.FastModel)
	cfg.SmartModel = prompt(reader, "  Smart model (generation)", cfg.SmartModel)

	fmt.Println()
	bold.Println("  Preferences:")
	gitignoreStr := prompt(reader, "  Auto-manage .gitignore (true/false)", fmt.Sprintf("%t", cfg.AutoGitignore))
	cfg.AutoGitignore = gitignoreStr == "true"

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save error: %w", err)
	}

	fmt.Println()
	green.Printf("  ✓ Config saved to %s\n", config.Path())
	return nil
}

func runConfSet(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Default()
	}

	key, value := args[0], args[1]

	switch strings.ToLower(key) {
	case "api-provider", "provider":
		cfg.APIProvider = value
	case "api-key", "key":
		cfg.APIKey = value
	case "fast-model":
		cfg.FastModel = value
	case "smart-model":
		cfg.SmartModel = value
	case "auto-gitignore", "gitignore":
		cfg.AutoGitignore = value == "true"
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	green.Printf("  ✓ %s = %s\n", key, value)
	return nil
}

func runConfGet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	key := strings.ToLower(args[0])
	switch key {
	case "api-provider", "provider":
		fmt.Println(cfg.APIProvider)
	case "api-key", "key":
		fmt.Println(maskKey(cfg.APIKey))
	case "fast-model":
		fmt.Println(cfg.FastModel)
	case "smart-model":
		fmt.Println(cfg.SmartModel)
	case "auto-gitignore", "gitignore":
		fmt.Println(cfg.AutoGitignore)
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
	return nil
}

func runConfShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no config found (run 'omnix conf')")
	}

	cyan.Println("  omnix configuration")
	fmt.Println()
	fmt.Printf("  %-18s %s\n", "API Provider:", cfg.APIProvider)
	fmt.Printf("  %-18s %s\n", "API Key:", maskKey(cfg.APIKey))
	fmt.Printf("  %-18s %s\n", "Fast Model:", cfg.FastModel)
	fmt.Printf("  %-18s %s\n", "Smart Model:", cfg.SmartModel)
	fmt.Printf("  %-18s %t\n", "Auto Gitignore:", cfg.AutoGitignore)
	fmt.Printf("  %-18s %s\n", "Config Path:", config.Path())
	fmt.Printf("  %-18s %s\n", "DB Path:", cfg.SQLitePath)
	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
