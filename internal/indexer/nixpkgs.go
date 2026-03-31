package indexer

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/banfen321/omnix/internal/storage"
)

type Indexer struct {
	db *storage.SQLiteDB
}

func New(db *storage.SQLiteDB) *Indexer {
	return &Indexer{
		db: db,
	}
}

type nixPkg struct {
	PName       string `json:"pname"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

func (idx *Indexer) IndexNixpkgs() (int, error) {
	out, err := exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "search", "nixpkgs", "^", "--json").Output()
	if err != nil {
		return 0, fmt.Errorf("nix search: %w", err)
	}

	var packages map[string]nixPkg
	if err := json.Unmarshal(out, &packages); err != nil {
		return 0, fmt.Errorf("parse nix search output: %w", err)
	}

	if err := idx.db.ClearNixpkgs(); err != nil {
		return 0, fmt.Errorf("clear nixpkgs: %w", err)
	}

	batchSize := 5000
	var batch []storage.NixPkgSearch
	total := 0

	for attr, pkg := range packages {
		// Clean quotes from description for FTS5 safety if needed
		batch = append(batch, storage.NixPkgSearch{
			NixAttr:     attr,
			PName:       pkg.PName,
			Version:     pkg.Version,
			Description: pkg.Description,
		})
		total++

		if len(batch) >= batchSize {
			if err := idx.db.InsertNixpkgsBatch(batch); err != nil {
				return total, fmt.Errorf("insert batch: %w", err)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := idx.db.InsertNixpkgsBatch(batch); err != nil {
			return total, fmt.Errorf("insert final batch: %w", err)
		}
	}

	return total, nil
}
