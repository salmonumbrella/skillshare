package utils

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// MarshalYAML marshals v with 2-space indentation (Go yaml default is 4).
// Every YAML file skillshare writes goes through it, so edits never reindent a user's file.
func MarshalYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
