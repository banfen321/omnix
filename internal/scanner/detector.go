package scanner

import (
	"os"
	"path/filepath"

	"github.com/banfen321/OmniNix/internal/scanner/parsers"
)

type Dependency struct {
	Name      string
	Version   string
	Ecosystem string
}

type ProjectInfo struct {
	Path         string
	Stacks       []string
	Dependencies []Dependency
}

type Detector struct {
	parsers []parsers.Parser
}

func NewDetector() *Detector {
	return &Detector{
		parsers: []parsers.Parser{
			&parsers.PythonParser{},
			&parsers.NodeParser{},
			&parsers.GoParser{},
			&parsers.RustParser{},
			&parsers.TerraformParser{},
			&parsers.DockerParser{},
			&parsers.JavaParser{},
			&parsers.PHPParser{},
			&parsers.RubyParser{},
			&parsers.KubernetesParser{},
			&parsers.ElixirParser{},
			&parsers.DotnetParser{},
			&parsers.BunParser{},
			&parsers.DenoParser{},
			&parsers.GenericParser{},
		},
	}
}

func (d *Detector) Scan(dir string) (*ProjectInfo, error) {
	info := &ProjectInfo{
		Path: dir,
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	for _, p := range d.parsers {
		if p.Detect(dir, files) {
			deps, err := p.Parse(dir)
			if err != nil {
				continue
			}

			if len(deps) > 0 {
				info.Stacks = append(info.Stacks, p.Name())
				for _, dep := range deps {
					info.Dependencies = append(info.Dependencies, Dependency{
						Name:      dep.Name,
						Version:   dep.Version,
						Ecosystem: dep.Ecosystem,
					})
				}
			}
		}
	}

	return info, nil
}
