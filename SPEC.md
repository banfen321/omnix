# omnix — Technical Specification

## 👋 Overview

Hey there! If you're looking to contribute to `omnix` or just want to know how the magic happens, you're in the right place. 

`omnix` is a Go CLI tool that takes the pain out of creating Nix development environments. Instead of making you manually write `flake.nix` files and hunt down which C-libraries your Python/Node/Rust modules need to compile, `omnix` scans your code, resolves the exact Nix packages you need, and generates a fully working environment for you.

## 🧱 Components

### 1. Scanner (`internal/scanner/`)

The **Detector** looks at the project folder. If it sees `requirements.txt`, `package.json`, or `go.mod`, it triggers the corresponding parser. If it doesn't see those, it looks for raw `.py`, `.js`, or `.go` source code files and triggers our "Fallback AST Scanners".

**Parsers** (10 total):

| Parser | Files | Output |
|--------|-------|--------|
| Python | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile` | `python3` + system C-libs (gcc, libxml2, openssl, etc.) |
| Node | `package.json` | `nodejs` + native build deps (if node-gyp detected) |
| Go | `go.mod` | `go` + gcc/pkg-config (if CGo detected) |
| Rust | `Cargo.toml` | `rustc`, `cargo` + system libs (if `-sys` crates detected) |
| Java | `pom.xml`, `build.gradle` | `jdk`, `maven`/`gradle` |
| PHP | `composer.json` | `php`, `composer` |
| Ruby | `Gemfile` | `ruby`, `bundler` |
| Terraform | `*.tf` | `terraform` |
| Docker | `Dockerfile`, `docker-compose.yml` | `docker`, `docker-compose` |
| Generic | `Makefile`, `.tool-versions` | detected tools |

**Key design**: Parsers do NOT emit library-level pip/npm packages into the flake.
Instead they detect which pip/npm packages require **system-level C libraries**
and emit those (e.g., `numpy` → `gcc`, `gfortran`; `lxml` → `libxml2`, `libxslt`).
Application-level deps are installed via shellHook (`pip install`, `npm ci`).

### 2. Resolver (`internal/resolver/`)

We have a 4-step fallback chain, fully parallelized via goroutines:

1. **Static Mapper** — A hardcoded `known_mappings.json` with 150+ pip/npm/system to Nix entries.
2. **Version DB** — A SQLite table of exact packages we've already synced.
3. **FTS5 Search** — Full-text search across 107k indexed nixpkgs.
4. **LLM Fallback** — OpenRouter/Google AI for those rare, weirdly-named edge cases.

### 3. Generator (`internal/generator/`)

Produces:
- `.nix/flake.nix` — Nix flake with `devShells.default`
- `.envrc` — `use flake ./.nix` for direnv integration

**shellHook** auto-installs project dependencies:
- Python: creates `.venv`, activates it, runs `pip install -r requirements.txt`
- Node: runs `npm ci` / `yarn install` / `pnpm install`
- Go: sets `GOPATH` and `PATH`
- Rust: sets `RUST_BACKTRACE=1`

### 4. Validator/Self-Healing Loop (`internal/validator/`)

Once the `flake.nix` is generated, `omnix` invokes a self-healing validation loop (up to 100 tries):
1. **Validation Check**: Evaluates the flake and captures error output.
2. **Instant Path**: If attributes were simply renamed in nixpkgs (e.g., `github3_py` → `github3-py`), it auto-corrects them instantly.
3. **Fast Path**: For missing Nix attributes, it evaluates context using the "fast model" (LLM) to either find the correct attribute or explicitly "SKIP" it (letting `pip`/`npm` handle it at runtime).
4. **Slow Path**: For complex compilation/syntax errors, it falls back to the "smart model" by passing the raw `flake.nix` and error trace for an AI-generated fix (limited to 1 attempt).

### 5. Storage (`internal/storage/`)

All data lives entirely locally on your machine in a single SQLite database at `~/.config/omnix/omnix.sqlite`. It contains:
- `cache` — Remembers what environments we've already generated (hashed via project files) so re-running `scan` is instant.
- `versions` — Stores recent exact package versions.
- `nixpkgs_fts` — The powerhouse! This is an FTS5 virtual table for lightning-fast full-text searches across Nixpkgs.

### 6. Indexer (`internal/indexer/`)

When you run `omnix sync`, the Indexer runs `nix search nixpkgs --json` under the hood. It downloads the massive Nixpkgs JSON index and inserts all ~107k+ packages directly into the SQLite FTS5 table. Thanks to Go's concurrency and SQLite's speed, this entire process usually takes just 3 to 5 seconds!

### Config (`internal/config/`)

TOML config at `~/.config/omnix/config.toml` (managed via `omnix conf`):

```toml
api_provider = "openrouter" # or "google"
api_key = "..."
fast_model = "google/gemini-3.1-flash-lite-preview"
smart_model = "anthropic/claude-sonnet-4.6"
auto_gitignore = true
sqlite_path = "~/.config/omnix/omnix.sqlite"
```

## Generated File Structure

```
project/
├── .nix/
│   ├── flake.nix      # Generated Nix flake
│   └── flake.lock     # Auto-generated lock file
├── .envrc             # direnv integration
└── (project files)
```

## Dependencies

- **Runtime**: Go binary (no runtime deps)
- **Build**: Go 1.21+, `modernc.org/sqlite` (pure-Go SQLite)
- **CLI**: `spf13/cobra`
- **Optional**: `direnv` (for auto-activation), `nix` (for environment)