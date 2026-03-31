package parsers

import "strings"

type DotnetParser struct{}

func (p *DotnetParser) Name() string { return "dotnet" }

func (p *DotnetParser) Detect(dir string, files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(f, ".csproj") || strings.HasSuffix(f, ".fsproj") || f == "global.json" {
			return true
		}
	}
	return false
}

func (p *DotnetParser) Parse(dir string) ([]Dep, error) {
	return []Dep{
		{Name: "dotnet-sdk", Ecosystem: "dotnet"},
	}, nil
}
