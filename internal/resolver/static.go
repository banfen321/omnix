package resolver

import (
	_ "embed"
	"encoding/json"
)

//go:embed known_mappings.json
var knownMappingsData []byte

type StaticMapper struct {
	mappings map[string]map[string]string
}

func NewStaticMapper() (*StaticMapper, error) {
	var data map[string]map[string]string
	if err := json.Unmarshal(knownMappingsData, &data); err != nil {
		return nil, err
	}
	return &StaticMapper{mappings: data}, nil
}

func (s *StaticMapper) Lookup(ecosystem, pkgName string) (string, bool) {
	ecoMap, ok := s.mappings[ecosystem]
	if !ok {
		return "", false
	}
	attr, ok := ecoMap[pkgName]
	return attr, ok
}
