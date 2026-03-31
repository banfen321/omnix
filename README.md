# 🧬 omnix — AI-Powered Nix Dev Environment Generator

> Just type `omnix scan` in your project folder, and watch your Nix environment build itself.

`omnix` is a smart, zero-config CLI tool that analyzes your code and builds a reproducible Nix development environment for you. It reads your project files (`requirements.txt`, `package.json`, `go.mod`, or even reads raw `.py` and `.js` source code!), figures out exactly what native C-libraries and tools you need, and generates a ready-to-use `.nix/flake.nix`. 

Best part? When you `cd` into the directory, your environment turns on. When you leave, it turns off. magic! 🪄

## 📦 Prerequisites

Before using `omnix`, you need two things installed on your system:
1. **[Nix](https://nixos.org/download)** — The package manager itself.
2. **direnv** — The magic tool that auto-activates environments when you change folders.
   ```bash
   nix profile install nixpkgs#direnv nixpkgs#nix-direnv
   ```
   *(Don't forget to add `eval "$(direnv hook zsh)"` to your `~/.zshrc` or bashrc!)*

## ⚡ Quick Start

### 1. Install `omnix`
**Using Nix (Recommended):**
```bash
nix --extra-experimental-features "nix-command flakes" profile install github:banfen321/omnix
```

**Using Go:**
```bash
go install github.com/banfen321/omnix@latest
```

### 2. Configure API Key
You'll need a free API key from OpenRouter or Google AI. This is only used as a smart fallback if `omnix` encounters a dependency it has never seen before.
```bash
omnix conf
```

### 3. Sync the Database
`omnix` runs entirely locally. It needs to know what packages exist in Nix. This command downloads the latest index from `nixpkgs` directly into a local SQLite database on your computer (`~/.config/omnix/omnix.sqlite`).
```bash
omnix sync
# Takes about 3-5 seconds to index 100k+ packages!
```

### 4. Build Your Environment!
Go to any project folder and let `omnix` do its thing:
```bash
cd my-cool-project/
omnix scan
```
That's it! `direnv` will automatically activate, and your environment is ready.

---

## 🧠 How it Works (Under the Hood)

When you run `scan`, here is what happens:
1. **Detection:** The tool looks at your project (Python, Node, Go, Rust, etc.).
2. **Deep Parsing:** It doesn't just look for `pip` or `npm`. It knows that if you import `numpy` or `canvas`, you need native C-libraries like `gcc`, `libjpeg`, and `pkg-config` to compile them. It figures this out automatically.
3. **AST Fallback:** No `requirements.txt`? No problem. The tool will parse your actual source code (like `import pandas`) to figure out what you're building.
4. **Local Resolution:** It searches your local SQLite database (`~/.config/omnix/omnix.sqlite`) in microseconds to find the exact Nix attributes for your dependencies.
5. **Generation:** It writes a minimalist `.nix/flake.nix` file and sets up a `shellHook` that will *automatically* run `pip install`, `npm ci`, etc., the moment you enter the folder!

## 📜 Commands

| Command | Description |
|---------|-------------|
| `omnix scan` | Scan project and generate `.nix/` environment |
| `omnix update` | Force re-scan (ignores cache) |
| `omnix sync` | Index nixpkgs into your local SQLite DB |
| `omnix conf` | Configure API keys and model settings |
| `omnix status` | Show current environment status |
| `omnix rm` | Remove generated `.nix/` files |

## 🏗️ Architecture

```
CLI → Scanner (Manifests/AST) → Resolver (Static Map → SQLite → LLM Fallback) → Generator
```
- **Database Location:** `~/.config/omnix/omnix.sqlite`
- **Config Location:** `~/.config/omnix/config.toml`

Everything stays on your machine. The LLM is only pinged if a package name is completely unknown, ensuring maximum speed and privacy.

## 📝 License
MIT License.