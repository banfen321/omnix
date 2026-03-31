package parsers

type BunParser struct{}

func (p *BunParser) Name() string { return "bun" }

func (p *BunParser) Detect(dir string, files []string) bool {
	return hasFile(files, "bun.lockb") || hasFile(files, "bunfig.toml")
}

func (p *BunParser) Parse(dir string) ([]Dep, error) {
	return []Dep{
		{Name: "bun", Ecosystem: "bun"},
	}, nil
}
