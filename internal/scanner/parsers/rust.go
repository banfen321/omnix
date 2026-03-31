package parsers

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type RustParser struct{}

func (p *RustParser) Name() string { return "rust" }

func (p *RustParser) Detect(dir string, files []string) bool {
	return hasFile(files, "Cargo.toml")
}

// rustSysDeps maps Rust crate names to Nix system packages they need
var rustSysDeps = map[string][]string{
	"openssl-sys":          {"openssl", "pkg-config"},
	"openssl":              {"openssl", "pkg-config"},
	"native-tls":           {"openssl", "pkg-config"},
	"libsqlite3-sys":       {"sqlite", "pkg-config"},
	"libz-sys":             {"zlib", "pkg-config"},
	"bzip2-sys":            {"bzip2", "pkg-config"},
	"curl-sys":             {"curl", "openssl", "pkg-config"},
	"libgit2-sys":          {"libgit2", "openssl", "pkg-config", "cmake"},
	"libssh2-sys":          {"libssh2", "openssl", "pkg-config", "cmake"},
	"pq-sys":               {"postgresql", "pkg-config"},
	"zmq-sys":              {"zeromq", "pkg-config"},
	"rdkafka-sys":          {"rdkafka", "pkg-config"},
	"rocksdb":              {"rocksdb", "gcc", "cmake"},
	"lmdb-sys":             {"lmdb", "pkg-config"},
	"freetype-sys":         {"freetype", "pkg-config"},
	"servo-fontconfig-sys": {"fontconfig", "pkg-config"},
	"alsa-sys":             {"alsa-lib", "pkg-config"},
	"dbus":                 {"dbus", "pkg-config"},
}

func (p *RustParser) Parse(dir string) ([]Dep, error) {
	deps := []Dep{
		{Name: "rustc", Ecosystem: "rust"},
		{Name: "cargo", Ecosystem: "rust"},
	}

	f, err := os.Open(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return deps, nil
	}
	defer f.Close()

	sysDeps := make(map[string]bool)
	inDeps := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[dependencies]" || line == "[dev-dependencies]" || line == "[build-dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDeps = false
			continue
		}
		if !inDeps || line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract crate name from: crate_name = "version" or crate_name = { ... }
		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}
		crateName := strings.TrimSpace(parts[0])

		if nixDeps, ok := rustSysDeps[crateName]; ok {
			for _, nd := range nixDeps {
				sysDeps[nd] = true
			}
		}
	}

	for nixPkg := range sysDeps {
		deps = append(deps, Dep{Name: nixPkg, Ecosystem: "system"})
	}

	return deps, nil
}
