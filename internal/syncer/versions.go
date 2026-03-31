package syncer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/banfen321/omnix/internal/config"
	"github.com/banfen321/omnix/internal/storage"
)

type Syncer struct {
	cfg  *config.Config
	db   *storage.SQLiteDB
	http *http.Client
}

func New(cfg *config.Config, db *storage.SQLiteDB) *Syncer {
	return &Syncer{
		cfg: cfg,
		db:  db,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Syncer) SyncAll() (map[string]int, error) {
	stats := make(map[string]int)
	var lastErr error

	if count, err := s.syncPyPI(); err != nil {
		lastErr = err
	} else {
		stats["python"] = count
	}

	if count, err := s.syncNpm(); err != nil {
		lastErr = err
	} else {
		stats["node"] = count
	}

	if count, err := s.syncGoProxy(); err != nil {
		lastErr = err
	} else {
		stats["go"] = count
	}

	return stats, lastErr
}

func (s *Syncer) syncPyPI() (int, error) {
	topPackages := []string{
		"flask", "django", "requests", "numpy", "pandas", "scipy",
		"matplotlib", "pillow", "sqlalchemy", "celery", "redis",
		"fastapi", "uvicorn", "gunicorn", "pytest", "black",
		"mypy", "ruff", "pyyaml", "boto3", "cryptography",
		"paramiko", "jinja2", "aiohttp", "httpx", "pydantic",
		"alembic", "psycopg2", "scrapy", "beautifulsoup4", "lxml",
		"click", "typer", "rich", "tqdm", "tornado", "sanic",
		"starlette", "websockets", "grpcio", "protobuf",
	}

	count := 0
	for _, pkg := range topPackages {
		url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkg)
		resp, err := s.http.Get(url)
		if err != nil || resp.StatusCode != 200 {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Info struct {
				Version string `json:"version"`
			} `json:"info"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		nixAttr := fmt.Sprintf("python3Packages.%s", pkg)
		s.db.UpsertVersion("python", pkg, result.Info.Version, nixAttr)
		count++
	}

	return count, nil
}

func (s *Syncer) syncNpm() (int, error) {
	topPackages := []string{
		"typescript", "eslint", "prettier", "webpack", "vite",
		"next", "react", "vue", "angular", "express", "fastify",
		"axios", "lodash", "moment", "dayjs", "tailwindcss",
		"jest", "vitest", "mocha", "nodemon", "pm2",
	}

	count := 0
	for _, pkg := range topPackages {
		url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", pkg)
		resp, err := s.http.Get(url)
		if err != nil || resp.StatusCode != 200 {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		nixAttr := fmt.Sprintf("nodePackages.%s", pkg)
		s.db.UpsertVersion("node", pkg, result.Version, nixAttr)
		count++
	}

	return count, nil
}

func (s *Syncer) syncGoProxy() (int, error) {
	topModules := []string{
		"github.com/gin-gonic/gin",
		"github.com/gorilla/mux",
		"github.com/labstack/echo",
		"github.com/spf13/cobra",
		"github.com/spf13/viper",
		"github.com/go-chi/chi",
		"github.com/stretchr/testify",
		"google.golang.org/grpc",
		"google.golang.org/protobuf",
		"github.com/sirupsen/logrus",
	}

	count := 0
	for _, mod := range topModules {
		url := fmt.Sprintf("https://proxy.golang.org/%s/@latest", mod)
		resp, err := s.http.Get(url)
		if err != nil || resp.StatusCode != 200 {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Version string `json:"Version"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		s.db.UpsertVersion("go", mod, result.Version, "go")
		count++
	}

	return count, nil
}
