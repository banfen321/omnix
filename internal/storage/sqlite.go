package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	db *sql.DB
}

type VersionEntry struct {
	Ecosystem string
	PkgName   string
	LatestVer string
	NixAttr   string
	UpdatedAt time.Time
}

func OpenSQLite(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

func (s *SQLiteDB) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS versions (
			ecosystem TEXT NOT NULL,
			pkg_name TEXT NOT NULL,
			latest_ver TEXT DEFAULT '',
			nix_attr TEXT DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (ecosystem, pkg_name)
		)`,
		`CREATE TABLE IF NOT EXISTS resolved_cache (
			project_hash TEXT PRIMARY KEY,
			flake_nix TEXT NOT NULL,
			deps_json TEXT DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_ecosystem ON versions(ecosystem)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_created ON resolved_cache(created_at)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS nixpkgs_fts USING fts5(
			nix_attr UNINDEXED,
			pname,
			version UNINDEXED,
			description
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

func (s *SQLiteDB) GetCache(hash string) (string, error) {
	var flake string
	err := s.db.QueryRow("SELECT flake_nix FROM resolved_cache WHERE project_hash = ?", hash).Scan(&flake)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("not found")
	}
	return flake, err
}

func (s *SQLiteDB) PutCache(hash, nixDir string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO resolved_cache (project_hash, flake_nix, created_at) VALUES (?, ?, ?)",
		hash, nixDir, time.Now().UTC(),
	)
	return err
}

func (s *SQLiteDB) UpsertVersion(ecosystem, pkgName, latestVer, nixAttr string) error {
	_, err := s.db.Exec(
		`INSERT INTO versions (ecosystem, pkg_name, latest_ver, nix_attr, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(ecosystem, pkg_name) DO UPDATE SET latest_ver=?, nix_attr=?, updated_at=?`,
		ecosystem, pkgName, latestVer, nixAttr, time.Now().UTC(),
		latestVer, nixAttr, time.Now().UTC(),
	)
	return err
}

func (s *SQLiteDB) GetVersion(ecosystem, pkgName string) (*VersionEntry, error) {
	var v VersionEntry
	err := s.db.QueryRow(
		"SELECT ecosystem, pkg_name, latest_ver, nix_attr, updated_at FROM versions WHERE ecosystem=? AND pkg_name=?",
		ecosystem, pkgName,
	).Scan(&v.Ecosystem, &v.PkgName, &v.LatestVer, &v.NixAttr, &v.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *SQLiteDB) GetVersionsByEcosystem(ecosystem string) ([]VersionEntry, error) {
	rows, err := s.db.Query(
		"SELECT ecosystem, pkg_name, latest_ver, nix_attr, updated_at FROM versions WHERE ecosystem=?",
		ecosystem,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []VersionEntry
	for rows.Next() {
		var v VersionEntry
		if err := rows.Scan(&v.Ecosystem, &v.PkgName, &v.LatestVer, &v.NixAttr, &v.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, v)
	}
	return entries, nil
}

func (s *SQLiteDB) CountVersions() (map[string]int, error) {
	rows, err := s.db.Query("SELECT ecosystem, COUNT(*) FROM versions GROUP BY ecosystem")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var eco string
		var count int
		if err := rows.Scan(&eco, &count); err != nil {
			return nil, err
		}
		counts[eco] = count
	}
	return counts, nil
}

func (s *SQLiteDB) SetMeta(key, value string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)",
		key, value,
	)
	return err
}

func (s *SQLiteDB) GetMeta(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteDB) ClearCache() error {
	_, err := s.db.Exec("DELETE FROM resolved_cache")
	return err
}

type NixPkgSearch struct {
	NixAttr     string
	PName       string
	Version     string
	Description string
}

func (s *SQLiteDB) InsertNixpkgsBatch(packages []NixPkgSearch) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO nixpkgs_fts (nix_attr, pname, version, description) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pkg := range packages {
		if _, err := stmt.Exec(pkg.NixAttr, pkg.PName, pkg.Version, pkg.Description); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteDB) ClearNixpkgs() error {
	_, err := s.db.Exec("DELETE FROM nixpkgs_fts")
	return err
}

func (s *SQLiteDB) SearchNixpkgs(query string, limit int) ([]NixPkgSearch, error) {
	rows, err := s.db.Query(`
		SELECT nix_attr, pname, version, description 
		FROM nixpkgs_fts 
		WHERE nixpkgs_fts MATCH ? 
		ORDER BY rank 
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NixPkgSearch
	for rows.Next() {
		var p NixPkgSearch
		if err := rows.Scan(&p.NixAttr, &p.PName, &p.Version, &p.Description); err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}
