package parsers

type DenoParser struct{}

func (p *DenoParser) Name() string { return "deno" }

func (p *DenoParser) Detect(dir string, files []string) bool {
	return hasFile(files, "deno.json") || hasFile(files, "deno.jsonc")
}

func (p *DenoParser) Parse(dir string) ([]Dep, error) {
	return []Dep{
		{Name: "deno", Ecosystem: "deno"},
	}, nil
}
