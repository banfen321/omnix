package parsers

import "path/filepath"

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
