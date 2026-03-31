package cmd

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/banfen321/OmniNix/internal/config"
	"github.com/banfen321/OmniNix/internal/indexer"
	"github.com/banfen321/OmniNix/internal/storage"
	"github.com/banfen321/OmniNix/internal/syncer"
	"github.com/spf13/cobra"
)

var syncAutoInterval string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync nixpkgs index and version database",
	Long:  "Update SQLite nixpkgs full-text search index and version database from upstream sources",
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().StringVar(&syncAutoInterval, "auto", "", "run sync automatically at interval (e.g., '7d', '24h')")
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config error: %w (run 'omnix conf' first)", err)
	}

	if syncAutoInterval != "" {
		return runAutoSync(cfg)
	}

	return runManualSync(cfg)
}

func runManualSync(cfg *config.Config) error {
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Prefix = "  "

	cyan.Println("  Syncing nixpkgs index and version database")
	fmt.Println()

	db, err := storage.OpenSQLite(cfg.SQLitePath)
	if err != nil {
		s.Stop()
		return fmt.Errorf("storage error: %w", err)
	}
	defer db.Close()

	s.Suffix = " Indexing nixpkgs into SQLite FTS5..."
	s.Start()

	idx := indexer.New(db)
	count, err := idx.IndexNixpkgs()
	if err != nil {
		s.Stop()
		return fmt.Errorf("nixpkgs indexing error: %w", err)
	}

	s.Stop()
	green.Printf("  ✓ Indexed %d nixpkgs packages into SQLite\n", count)

	s.Suffix = " Syncing version database..."
	s.Start()

	syn := syncer.New(cfg, db)
	stats, err := syn.SyncAll()
	if err != nil {
		s.Stop()
		yellow.Printf("  ⚠ Partial sync: %s\n", err)
	} else {
		s.Stop()
		green.Println("  ✓ Version database synced")
	}

	fmt.Println()
	dim.Println("  Sync results:")
	for eco, count := range stats {
		dim.Printf("    %-12s %d packages\n", eco+":", count)
	}

	db.SetMeta("last_sync", time.Now().UTC().Format(time.RFC3339))

	return nil
}

func runAutoSync(cfg *config.Config) error {
	duration, err := parseDuration(syncAutoInterval)
	if err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	cyan.Printf("  Auto-sync enabled: every %s\n", duration)
	dim.Println("  Press Ctrl+C to stop")

	for {
		if err := runManualSync(cfg); err != nil {
			yellow.Printf("  ⚠ Sync error: %s\n", err)
		}
		fmt.Println()
		dim.Printf("  Next sync in %s...\n", duration)
		time.Sleep(duration)
	}
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("too short")
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, err
	}

	switch unit {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	default:
		return time.ParseDuration(s)
	}
}
