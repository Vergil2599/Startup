package plugin

import "gopkg.in/yaml.v3"

// Indirección mínima para no repetir imports en manifest.go.
func yamlUnmarshal(data []byte, v any) error { return yaml.Unmarshal(data, v) }
func yamlMarshal(v any) ([]byte, error)      { return yaml.Marshal(v) }
