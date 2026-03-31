package parsers

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type NodeParser struct{}

func (p *NodeParser) Name() string { return "node" }

func (p *NodeParser) Detect(dir string, files []string) bool {
	if hasFile(files, "package.json") {
		return true
	}
	// Fallback
	for _, f := range files {
		if strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".ts") {
			return true
		}
	}
	return false
}

// nodeNativePkgs are npm packages that require native build tools
var nodeNativePkgs = map[string][]string{
	"node-gyp":       {"gcc", "gnumake", "python3"},
	"node-pre-gyp":   {"gcc", "gnumake", "python3"},
	"node-sass":      {"gcc", "gnumake", "python3"},
	"bcrypt":         {"gcc", "gnumake", "python3"},
	"sharp":          {"gcc", "pkg-config", "vips"},
	"canvas":         {"gcc", "pkg-config", "cairo", "pango", "libjpeg"},
	"sqlite3":        {"gcc", "gnumake", "python3"},
	"better-sqlite3": {"gcc", "gnumake", "python3"},
	"libpq":          {"gcc", "postgresql"},
	"pg-native":      {"gcc", "postgresql"},
	"grpc":           {"gcc", "cmake"},
	"@grpc/grpc-js":  {},
	"bufferutil":     {"gcc", "gnumake"},
	"utf-8-validate": {"gcc", "gnumake"},
	"fsevents":       {},
	"esbuild":        {},
	"swc":            {},
	"re2":            {"gcc", "gnumake", "python3"},
	"leveldown":      {"gcc", "gnumake", "python3"},
	"zeromq":         {"gcc", "pkg-config", "zeromq"},
}

// Common Node.js stdlib modules
var nodeStdlib = map[string]bool{
	"fs": true, "path": true, "http": true, "https": true, "crypto": true, "os": true,
	"util": true, "events": true, "stream": true, "net": true, "child_process": true,
	"cluster": true, "url": true, "querystring": true, "assert": true, "buffer": true,
	"dns": true, "tls": true, "zlib": true, "readline": true, "dgram": true, "v8": true,
	"vm": true, "tty": true, "worker_threads": true, "perf_hooks": true, "async_hooks": true,
}

func (p *NodeParser) Parse(dir string) ([]Dep, error) {
	deps := []Dep{
		{Name: "nodejs", Ecosystem: "node"},
	}

	sysDeps := make(map[string]bool)
	allNpmPkgs := make(map[string]bool)

	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err == nil {
		var pkg struct {
			Engines      map[string]string `json:"engines"`
			Dependencies map[string]string `json:"dependencies"`
			DevDeps      map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			for name := range pkg.Dependencies {
				allNpmPkgs[name] = true
			}
			for name := range pkg.DevDeps {
				allNpmPkgs[name] = true
			}
		}
	}

	// FALLBACK: If no npm packages found, scan .js/.ts files
	if len(allNpmPkgs) == 0 {
		extracted := extractNodeImports(dir)
		for _, pkg := range extracted {
			allNpmPkgs[pkg] = true
		}
	}

	for npmPkg := range allNpmPkgs {
		if nixDeps, ok := nodeNativePkgs[npmPkg]; ok {
			for _, nd := range nixDeps {
				sysDeps[nd] = true
			}
		}
		// Also output the raw application dependencies! The resolver will try to find them.
		if !nodeStdlib[npmPkg] {
			deps = append(deps, Dep{Name: npmPkg, Ecosystem: "node"})
		}
	}

	// Add detected system deps
	for nixPkg := range sysDeps {
		deps = append(deps, Dep{Name: nixPkg, Ecosystem: "system"})
	}

	return deps, nil
}

// extractNodeImports scans .js and .ts files for `require('X')` and `import ... from 'X'`
func extractNodeImports(dir string) []string {
	imports := make(map[string]bool)
	requireRegex := regexp.MustCompile(`require\(['"]([^.'"][^'"]+)['"]\)`)
	importRegex := regexp.MustCompile(`import\s+.*from\s+['"]([^.'"][^'"]+)['"]`)
	importStandaloneRegex := regexp.MustCompile(`import\s+['"]([^.'"][^'"]+)['"]`)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && (info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == "build") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".jsx") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			// match require('pkg')
			if matches := requireRegex.FindAllStringSubmatch(line, -1); matches != nil {
				for _, m := range matches {
					pkg := strings.Split(m[1], "/")[0]
					if !nodeStdlib[pkg] && !strings.HasPrefix(pkg, "node:") {
						imports[pkg] = true
					}
				}
			}
			// match import ... from 'pkg'
			if matches := importRegex.FindAllStringSubmatch(line, -1); matches != nil {
				for _, m := range matches {
					pkg := strings.Split(m[1], "/")[0]
					if !nodeStdlib[pkg] && !strings.HasPrefix(pkg, "node:") {
						imports[pkg] = true
					}
				}
			}
			// match import 'pkg'
			if matches := importStandaloneRegex.FindAllStringSubmatch(line, -1); matches != nil {
				for _, m := range matches {
					pkg := strings.Split(m[1], "/")[0]
					if !nodeStdlib[pkg] && !strings.HasPrefix(pkg, "node:") {
						imports[pkg] = true
					}
				}
			}
		}
		return nil
	})

	var result []string
	for imp := range imports {
		if imp != "" {
			result = append(result, imp)
		}
	}
	return result
}

func extractSemver(spec string) string {
	cleaned := spec
	for _, prefix := range []string{"^", "~", ">=", "<=", ">", "<", "="} {
		cleaned = trimPrefix(cleaned, prefix)
	}
	cleaned = trimSuffix(cleaned, ".x")

	if isVersion(cleaned) {
		return cleaned
	}
	return ""
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func isVersion(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}
