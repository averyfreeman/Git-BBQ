package gitbbq

import (
	"encoding/json"
	"path/filepath"
	"sort"

	"github.com/google/jsonschema-go/jsonschema"
)

func GenerateSchemas() (map[string][]byte, error) {
	values := []struct {
		name  string
		value any
	}{
		{name: "gitbbq-manifest.schema.json", value: Manifest{}},
		{name: "githabits.schema.json", value: GitHabits{}},
		{name: "githabits-plan.schema.json", value: GitPlan{}},
		{name: "githabits-execution.schema.json", value: GitExecution{}},
		{name: "architecture-contract.schema.json", value: ArchitectureContract{}},
	}
	result := make(map[string][]byte, len(values))
	for _, item := range values {
		var schema *jsonschema.Schema
		var err error
		switch item.value.(type) {
		case Manifest:
			schema, err = jsonschema.For[Manifest](nil)
		case GitHabits:
			schema, err = jsonschema.For[GitHabits](nil)
		case GitPlan:
			schema, err = jsonschema.For[GitPlan](nil)
		case GitExecution:
			schema, err = jsonschema.For[GitExecution](nil)
		case ArchitectureContract:
			schema, err = jsonschema.For[ArchitectureContract](nil)
		}
		if err != nil {
			return nil, err
		}
		schema.Schema = "https://json-schema.org/draft/2020-12/schema"
		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return nil, err
		}
		result[item.name] = append(data, '\n')
	}
	return result, nil
}

func WriteSchemas(root string) ([]string, error) {
	schemas, err := GenerateSchemas()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(schemas))
	for name, data := range schemas {
		relative := filepath.ToSlash(filepath.Join("schemas", "gitbbq", name))
		if _, err := writeGenerated(root, relative, data, true); err != nil {
			return nil, err
		}
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	return paths, nil
}
