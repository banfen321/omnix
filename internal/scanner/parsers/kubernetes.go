package parsers

import "os"

type KubernetesParser struct{}

func (p *KubernetesParser) Name() string { return "kubernetes" }

func (p *KubernetesParser) Detect(dir string, files []string) bool {
	return hasFile(files, "Chart.yaml") || hasFile(files, "helmfile.yaml") ||
		hasFile(files, "kustomization.yaml") || hasFile(files, "kustomization.yml")
}

func (p *KubernetesParser) Parse(dir string) ([]Dep, error) {
	deps := []Dep{
		{Name: "kubectl", Ecosystem: "system"},
	}

	entries, err := os.ReadDir(dir)
	if err == nil {
		var filenames []string
		for _, e := range entries {
			if !e.IsDir() {
				filenames = append(filenames, e.Name())
			}
		}
		if hasFile(filenames, "Chart.yaml") || hasFile(filenames, "helmfile.yaml") {
			deps = append(deps, Dep{Name: "kubernetes-helm", Ecosystem: "system"})
		}
		if hasFile(filenames, "kustomization.yaml") || hasFile(filenames, "kustomization.yml") {
			deps = append(deps, Dep{Name: "kustomize", Ecosystem: "system"})
		}
	}

	return deps, nil
}
