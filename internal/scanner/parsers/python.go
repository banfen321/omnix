package parsers

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PythonParser struct{}

func (p *PythonParser) Name() string { return "python" }

func (p *PythonParser) Detect(dir string, files []string) bool {
	targets := []string{"requirements.txt", "pyproject.toml", "setup.py", "setup.cfg", "Pipfile"}
	for _, t := range targets {
		if hasFile(files, t) {
			return true
		}
	}
	// Fallback: trigger Python parser if ANY .py file exists at root
	for _, f := range files {
		if strings.HasSuffix(f, ".py") {
			return true
		}
	}
	return false
}

// pythonSysDeps maps pip package names to Nix system packages they need for compilation.
var pythonSysDeps = map[string][]string{
	// Scientific / Numeric
	"numpy":        {"gcc", "gfortran", "pkg-config"},
	"scipy":        {"gcc", "gfortran", "pkg-config", "openblas"},
	"pandas":       {"gcc", "gfortran", "pkg-config"},
	"scikit-learn": {"gcc", "gfortran", "pkg-config", "openblas"},
	"scikit-image": {"gcc", "pkg-config"},
	"h5py":         {"gcc", "hdf5", "pkg-config"},
	"tables":       {"gcc", "hdf5", "pkg-config"},
	"pyarrow":      {"gcc", "cmake", "pkg-config"},
	"polars":       {"gcc", "cmake", "pkg-config"},

	// Image / Media
	"pillow":                 {"gcc", "zlib", "libjpeg", "libpng", "libtiff", "lcms2", "pkg-config"},
	"opencv-python":          {"gcc", "cmake", "pkg-config", "ffmpeg"},
	"opencv-python-headless": {"gcc", "cmake", "pkg-config", "ffmpeg"},

	// XML / HTML
	"lxml": {"gcc", "libxml2", "libxslt", "pkg-config"},

	// Cryptography / Networking
	"cryptography": {"gcc", "openssl", "pkg-config", "libffi"},
	"pyopenssl":    {"gcc", "openssl", "pkg-config"},
	"pynacl":       {"gcc", "libsodium", "pkg-config"},
	"bcrypt":       {"gcc", "libffi"},
	"paramiko":     {"gcc", "openssl", "pkg-config"},

	// Database
	"psycopg2":        {"gcc", "postgresql", "pkg-config"},
	"psycopg2-binary": {"gcc", "postgresql", "pkg-config"},
	"mysqlclient":     {"gcc", "mysql80", "pkg-config"},
	"pymssql":         {"gcc", "freetds", "pkg-config"},

	// Compression
	"zstandard":     {"gcc", "zstd", "pkg-config"},
	"lz4":           {"gcc", "lz4", "pkg-config"},
	"brotli":        {"gcc", "brotli", "pkg-config"},
	"python-snappy": {"gcc", "snappy", "pkg-config"},

	// Misc native
	"cffi":       {"gcc", "libffi", "pkg-config"},
	"greenlet":   {"gcc"},
	"gevent":     {"gcc", "libev"},
	"uvloop":     {"gcc", "libuv", "pkg-config"},
	"pyyaml":     {"gcc", "libyaml"},
	"markupsafe": {"gcc"},
	"msgpack":    {"gcc"},
	"ujson":      {"gcc"},
	"orjson":     {"gcc"},
	"regex":      {"gcc"},

	// Build backends (detected in pyproject.toml)
	"cython":          {"gcc", "python3Packages.cython"},
	"meson-python":    {"meson", "ninja", "pkg-config"},
	"meson":           {"meson", "ninja"},
	"cmake":           {"cmake"},
	"pybind11":        {"gcc", "cmake"},
	"swig":            {"swig", "gcc"},
	"setuptools-rust": {"rustc", "cargo"},

	// ML / AI
	"torch":      {"gcc", "cmake", "pkg-config"},
	"tensorflow": {"gcc", "cmake", "pkg-config"},
	"tokenizers": {"rustc", "cargo"},

	// System
	"dbus-python": {"gcc", "dbus", "pkg-config"},
	"pygobject":   {"gcc", "gobject-introspection", "pkg-config", "cairo"},
	"pycairo":     {"gcc", "cairo", "pkg-config"},
}

// Common Python stdlib modules to ignore during AST fallback scanning
var pythonStdlib = map[string]bool{
	"os": true, "sys": true, "json": true, "math": true, "re": true, "datetime": true,
	"time": true, "enum": true, "collections": true, "typing": true, "pathlib": true,
	"itertools": true, "functools": true, "subprocess": true, "shutil": true, "logging": true,
	"threading": true, "multiprocessing": true, "io": true, "sqlite3": true, "csv": true,
	"urllib": true, "hashlib": true, "random": true, "asyncio": true, "socket": true,
	"struct": true, "base64": true, "argparse": true, "uuid": true, "warnings": true,
}

func (p *PythonParser) Parse(dir string) ([]Dep, error) {
	// Always need python3 and pip
	deps := []Dep{
		{Name: "python3", Ecosystem: "python"},
	}

	// Collect all pip packages from manifest files
	var pipPkgs []string

	if d, err := parseRequirementsTxt(filepath.Join(dir, "requirements.txt")); err == nil {
		for _, dep := range d {
			pipPkgs = append(pipPkgs, dep.Name)
		}
	}

	if d, err := parsePyprojectToml(filepath.Join(dir, "pyproject.toml")); err == nil {
		for _, dep := range d {
			pipPkgs = append(pipPkgs, dep.Name)
		}
	}

	if d, err := parsePipfile(filepath.Join(dir, "Pipfile")); err == nil {
		for _, dep := range d {
			pipPkgs = append(pipPkgs, dep.Name)
		}
	}

	// Also check pyproject.toml build-system requirements
	if buildDeps, err := parsePyprojectBuildDeps(filepath.Join(dir, "pyproject.toml")); err == nil {
		pipPkgs = append(pipPkgs, buildDeps...)
	}

	// Check setup.py for install_requires
	if d, err := parseSetupPy(filepath.Join(dir, "setup.py")); err == nil {
		for _, dep := range d {
			pipPkgs = append(pipPkgs, dep.Name)
		}
	}

	// FALLBACK: If no manifests found (or they are empty), scan .py files recursively for imports
	if len(pipPkgs) == 0 {
		pipPkgs = extractPythonImports(dir)
	}

	// Resolve pip packages to system-level Nix deps
	sysDeps := make(map[string]bool)
	for _, pkg := range pipPkgs {
		normalized := strings.ToLower(strings.ReplaceAll(pkg, "_", "-"))
		if IsLocalModule(dir, normalized) || IsLocalModule(dir, pkg) {
			continue
		}
		if nixDeps, ok := pythonSysDeps[normalized]; ok {
			for _, nd := range nixDeps {
				sysDeps[nd] = true
			}
		}
		// Also output the raw application dependencies! The resolver will try to find them.
		if !pythonStdlib[normalized] {
			deps = append(deps, Dep{Name: normalized, Ecosystem: "python"})
		}
	}

	// Add detected system deps
	for nixPkg := range sysDeps {
		deps = append(deps, Dep{Name: nixPkg, Ecosystem: "system"})
	}

	return dedup(deps), nil
}

// extractPythonImports recursively scans .py files for `import X` and `from Y import Z`
func extractPythonImports(dir string) []string {
	imports := make(map[string]bool)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".py") {
			if info != nil && info.IsDir() && (info.Name() == ".venv" || info.Name() == "venv" || info.Name() == "__pycache__") {
				return filepath.SkipDir
			}
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
			if strings.HasPrefix(line, "import ") {
				parts := strings.Split(line, " ")
				if len(parts) >= 2 {
					pkg := strings.Split(parts[1], ".")[0]
					pkg = strings.Trim(pkg, "';\",")
					if pkg != "" && !pythonStdlib[pkg] && !IsLocalModule(dir, pkg) {
						imports[pkg] = true
					}
				}
			} else if strings.HasPrefix(line, "from ") {
				parts := strings.Split(line, " ")
				if len(parts) >= 2 {
					pkg := strings.Split(parts[1], ".")[0]
					pkg = strings.Trim(pkg, "';\",")
					if pkg != "" && !pythonStdlib[pkg] && !strings.HasPrefix(pkg, ".") && !IsLocalModule(dir, pkg) {
						imports[pkg] = true
					}
				}
			}
		}
		return nil
	})

	var result []string
	for imp := range imports {
		result = append(result, imp)
	}
	return result
}

// parsePyprojectBuildDeps extracts build-system.requires from pyproject.toml
func parsePyprojectBuildDeps(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var deps []string
	content := string(data)
	inBuildReqs := false
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "requires") && strings.Contains(line, "[") {
			inBuildReqs = true
			// Try to extract inline packages: requires = ["meson-python", "cython"]
			if idx := strings.Index(line, "["); idx >= 0 {
				rest := line[idx:]
				rest = strings.Trim(rest, "[]")
				parts := strings.Split(rest, ",")
				for _, p := range parts {
					p = strings.Trim(strings.TrimSpace(p), `"'`)
					if p == "" {
						continue
					}
					// Strip version specifiers
					name := extractPkgName(p)
					if name != "" {
						deps = append(deps, name)
					}
				}
				if strings.Contains(line, "]") {
					inBuildReqs = false
				}
			}
			continue
		}

		if inBuildReqs {
			if strings.Contains(line, "]") {
				inBuildReqs = false
			}
			cleaned := strings.Trim(line, `"', `)
			name := extractPkgName(cleaned)
			if name != "" {
				deps = append(deps, name)
			}
		}
	}

	return deps, nil
}

// parseSetupPy does a best-effort scan of setup.py for install_requires
func parseSetupPy(path string) ([]Dep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var deps []Dep
	content := string(data)
	inReqs := false
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.Contains(line, "install_requires") && strings.Contains(line, "[") {
			inReqs = true
			continue
		}
		if inReqs {
			if strings.Contains(line, "]") {
				inReqs = false
				continue
			}
			cleaned := strings.Trim(line, `"', `)
			name := extractPkgName(cleaned)
			if name != "" {
				deps = append(deps, Dep{Name: name, Ecosystem: "python"})
			}
		}
	}

	return deps, nil
}

// extractPkgName strips version specifiers from a package name
func extractPkgName(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	// Split on common version specifier chars
	for _, sep := range []string{">=", "<=", "!=", "==", "~=", ">", "<", ";"} {
		if idx := strings.Index(spec, sep); idx > 0 {
			spec = spec[:idx]
		}
	}
	// Remove extras like [security]
	if idx := strings.Index(spec, "["); idx > 0 {
		spec = spec[:idx]
	}
	return strings.TrimSpace(strings.ToLower(spec))
}

var reqLineRegex = regexp.MustCompile(`^([a-zA-Z0-9._-]+)\s*([><!=~]+\s*[0-9a-zA-Z.*]+(?:\s*,\s*[><!=~]+\s*[0-9a-zA-Z.*]+)*)?`)

func parseRequirementsTxt(path string) ([]Dep, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []Dep
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}

		m := reqLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		name := strings.ToLower(m[1])
		version := ""
		if m[2] != "" {
			version = extractExactVersion(m[2])
		}

		deps = append(deps, Dep{
			Name:      name,
			Version:   version,
			Ecosystem: "python",
		})
	}

	return deps, nil
}

func parsePyprojectToml(path string) ([]Dep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var deps []Dep
	content := string(data)

	inDeps := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "dependencies = [" || line == "[project.dependencies]" {
			inDeps = true
			continue
		}
		if inDeps {
			if line == "]" || (strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[\"")) {
				inDeps = false
				continue
			}

			cleaned := strings.Trim(line, `"', `)
			if cleaned == "" {
				continue
			}

			m := reqLineRegex.FindStringSubmatch(cleaned)
			if m == nil {
				continue
			}

			name := strings.ToLower(m[1])
			version := ""
			if m[2] != "" {
				version = extractExactVersion(m[2])
			}

			deps = append(deps, Dep{
				Name:      name,
				Version:   version,
				Ecosystem: "python",
			})
		}
	}

	return deps, nil
}

func parsePipfile(path string) ([]Dep, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []Dep
	inPackages := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[packages]" {
			inPackages = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPackages = false
			continue
		}
		if !inPackages || line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		verSpec := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		version := ""
		if verSpec != "*" {
			version = extractExactVersion(verSpec)
		}

		deps = append(deps, Dep{
			Name:      strings.ToLower(name),
			Version:   version,
			Ecosystem: "python",
		})
	}

	return deps, nil
}

func extractExactVersion(spec string) string {
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "==") {
		return strings.TrimSpace(spec[2:])
	}
	if strings.HasPrefix(spec, ">=") {
		parts := strings.Split(spec, ",")
		return strings.TrimSpace(strings.TrimPrefix(parts[0], ">="))
	}
	re := regexp.MustCompile(`(\d+\.\d+[\d.]*)`)
	m := re.FindString(spec)
	return m
}

func dedup(deps []Dep) []Dep {
	seen := make(map[string]bool)
	var result []Dep
	for _, d := range deps {
		key := d.Ecosystem + ":" + d.Name
		if !seen[key] {
			seen[key] = true
			result = append(result, d)
		}
	}
	return result
}
