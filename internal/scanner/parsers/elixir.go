package parsers

type ElixirParser struct{}

func (p *ElixirParser) Name() string { return "elixir" }

func (p *ElixirParser) Detect(dir string, files []string) bool {
	return hasFile(files, "mix.exs")
}

func (p *ElixirParser) Parse(dir string) ([]Dep, error) {
	return []Dep{
		{Name: "elixir", Ecosystem: "elixir"},
	}, nil
}
