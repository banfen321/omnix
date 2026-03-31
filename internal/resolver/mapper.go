package resolver

import (
	"fmt"
	"sync"

	"github.com/banfen321/omnix/internal/config"
	"github.com/banfen321/omnix/internal/scanner"
	"github.com/banfen321/omnix/internal/storage"
)

type NixPackage struct {
	OriginalName string
	NixAttr      string
	Version      string
	Ecosystem    string
	Source       string
}

type Resolver struct {
	cfg    *config.Config
	db     *storage.SQLiteDB
	static *StaticMapper
	llm    *LLMClient
}

func New(cfg *config.Config, db *storage.SQLiteDB) (*Resolver, error) {
	static, err := NewStaticMapper()
	if err != nil {
		static = &StaticMapper{mappings: make(map[string]map[string]string)}
	}

	return &Resolver{
		cfg:    cfg,
		db:     db,
		static: static,
		llm:    NewLLMClient(cfg),
	}, nil
}

func (r *Resolver) Resolve(deps []scanner.Dependency) ([]NixPackage, error) {
	resolved := make([]NixPackage, len(deps))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, dep := range deps {
		wg.Add(1)
		go func(index int, d scanner.Dependency) {
			defer wg.Done()
			pkg, err := r.resolveOne(d)

			if err != nil {
				pkg = &NixPackage{
					OriginalName: d.Name,
					NixAttr:      d.Name,
					Version:      d.Version,
					Ecosystem:    d.Ecosystem,
					Source:       "fallback",
				}
			}

			mu.Lock()
			resolved[index] = *pkg
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}(i, dep)
	}

	wg.Wait()
	return resolved, nil
}

func (r *Resolver) resolveOne(dep scanner.Dependency) (*NixPackage, error) {
	if attr, ok := r.static.Lookup(dep.Ecosystem, dep.Name); ok {
		return &NixPackage{
			OriginalName: dep.Name,
			NixAttr:      attr,
			Version:      dep.Version,
			Ecosystem:    dep.Ecosystem,
			Source:       "static",
		}, nil
	}

	if v, err := r.db.GetVersion(dep.Ecosystem, dep.Name); err == nil && v.NixAttr != "" {
		return &NixPackage{
			OriginalName: dep.Name,
			NixAttr:      v.NixAttr,
			Version:      dep.Version,
			Ecosystem:    dep.Ecosystem,
			Source:       "version_db",
		}, nil
	}

	query := fmt.Sprintf(`"%s" AND "%s"`, dep.Name, dep.Ecosystem)
	if dep.Ecosystem == "python" {
		query = fmt.Sprintf(`"%s" AND "python3"`, dep.Name)
	}

	results, err := r.db.SearchNixpkgs(query, 5)
	if err == nil && len(results) > 0 {
		candidates := make([]string, 0, len(results))
		for _, res := range results {
			if res.NixAttr == dep.Name || res.PName == dep.Name {
				return &NixPackage{
					OriginalName: dep.Name,
					NixAttr:      res.NixAttr,
					Version:      dep.Version,
					Ecosystem:    dep.Ecosystem,
					Source:       "fts5_exact",
				}, nil
			}
			candidates = append(candidates, res.NixAttr)
		}

		if len(candidates) > 0 {
			attr, err := r.llm.ResolvePackage(dep.Name, dep.Ecosystem, candidates)
			if err == nil && attr != "" {
				return &NixPackage{
					OriginalName: dep.Name,
					NixAttr:      attr,
					Version:      dep.Version,
					Ecosystem:    dep.Ecosystem,
					Source:       "llm",
				}, nil
			}
		}
	}

	attr, err := r.llm.ResolvePackage(dep.Name, dep.Ecosystem, nil)
	if err == nil && attr != "" {
		return &NixPackage{
			OriginalName: dep.Name,
			NixAttr:      attr,
			Version:      dep.Version,
			Ecosystem:    dep.Ecosystem,
			Source:       "llm_direct",
		}, nil
	}

	return nil, fmt.Errorf("could not resolve %s:%s", dep.Ecosystem, dep.Name)
}

func (r *Resolver) FixFlake(flakeContent, errorMsg string) (string, error) {
	return r.llm.FixFlake(flakeContent, errorMsg)
}
