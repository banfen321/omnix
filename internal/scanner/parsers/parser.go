package parsers

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Dep struct {
	Name      string
	Version   string
	Ecosystem string
}

type Parser interface {
	Name() string
	Detect(dir string, files []string) bool
	Parse(dir string) ([]Dep, error)
}

var (
	localModCache = make(map[string]map[string]bool)
	localModMu    sync.RWMutex
)

// IsLocalModule checks if a name corresponds to a local file or directory (up to 4 levels deep)
func IsLocalModule(dir, pkg string) bool {
	dir = filepath.Clean(dir)
	localModMu.RLock()
	cache, ok := localModCache[dir]
	localModMu.RUnlock()

	if !ok {
		// Index first 4 levels
		cache = make(map[string]bool)
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(dir, path)
			if rel == "." {
				return nil
			}

			// Don't go deeper than 4 levels
			if strings.Count(rel, string(os.PathSeparator)) >= 4 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden dirs and common noise
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "venv" || d.Name() == "dist" || d.Name() == "target" || d.Name() == "__pycache__" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			name := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			lowName := strings.ToLower(name)
			cache[lowName] = true
			cache[strings.ReplaceAll(lowName, "_", "-")] = true
			cache[strings.ReplaceAll(lowName, "-", "_")] = true

			return nil
		})

		localModMu.Lock()
		localModCache[dir] = cache
		localModMu.Unlock()
	}

	pkg = strings.ToLower(pkg)
	return cache[pkg]
}

func hasFile(files []string, name string) bool {
	for _, f := range files {
		base := filepath.Base(f)
		if base == name {
			return true
		}
	}
	return false
}

func hasFileWithExt(files []string, ext string) bool {
	for _, f := range files {
		if filepath.Ext(f) == ext {
			return true
		}
	}
	return false
}
