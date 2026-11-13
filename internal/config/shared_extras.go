package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/sharedfile"
)

// sharedExtraPaths is exhaustive; file/as are plain filenames, not paths.
var sharedExtraPaths = []string{"source", "targets[].path", "targets[].extension"}

func decodeSharedExtras(doc *sharedfile.Document) ([]ExtraConfig, *sharedfile.Unavailable) {
	var entries []ExtraConfig
	section := doc.Section("extras")
	if section == nil {
		return entries, nil
	}
	for _, node := range section.Content {
		var extra ExtraConfig
		err := node.Decode(&extra)
		if err == nil {
			err = ValidateExtraName(extra.Name)
		}
		if err == nil {
			err = ValidateExtraConfig(extra)
		}
		if err == nil {
			err = ValidateExtraNameUnique(extra.Name, entries)
		}
		if err != nil {
			// Validation errors can quote paths, modes, names or YAML scalar values.
			return nil, &sharedfile.Unavailable{Path: doc.Path, Reason: sharedfile.Entry, Findings: []sharedfile.Finding{{Line: node.Line, Column: node.Column}}}
		}
		entries = append(entries, extra)
	}
	return entries, nil
}

func resolveSharedExtras(dir string, declared []ExtraConfig, warnings *[]string) []ExtraConfig {
	resolved := make([]ExtraConfig, len(declared))
	for i, entry := range declared {
		extra := cloneSharedExtra(entry)
		resolve := func(field, p string) string {
			if !sharedfile.IsPortablePath(p) {
				*warnings = append(*warnings, fmt.Sprintf("extras.%s.%s: not portable", entry.Name, field))
			}
			return sharedfile.ResolvePath(dir, p)
		}
		extra.Source = resolve(sharedExtraPaths[0], extra.Source)
		for j := range extra.Targets {
			target := &extra.Targets[j]
			target.Path = resolve(sharedExtraPaths[1], target.Path)
			if strings.ContainsAny(target.Extension, `/\`) || strings.HasPrefix(target.Extension, ".") {
				target.Extension = resolve(sharedExtraPaths[2], target.Extension)
			}
		}
		resolved[i] = extra
	}
	return resolved
}

func cloneSharedExtra(e ExtraConfig) ExtraConfig {
	if e.Targets != nil {
		targets := make([]ExtraTargetConfig, len(e.Targets))
		copy(targets, e.Targets)
		e.Targets = targets
	}
	for i := range e.Targets {
		if e.Targets[i].Include != nil {
			e.Targets[i].Include = append([]string{}, e.Targets[i].Include...)
		}
		if e.Targets[i].Exclude != nil {
			e.Targets[i].Exclude = append([]string{}, e.Targets[i].Exclude...)
		}
	}
	return e
}

// SharedExtraDeclaration returns a detached declaration node, never resolved paths.
func (c *Config) SharedExtraDeclaration(name string) (yaml.Node, bool) {
	if c.sharedLayer == nil {
		return yaml.Node{}, false
	}
	node, ok := c.sharedLayer.declarations[name]
	if !ok {
		return yaml.Node{}, false
	}
	// Round-trip only the node to avoid exposing the load-time ownership snapshot.
	raw, err := yaml.Marshal(&node)
	if err != nil {
		return yaml.Node{}, false
	}
	var copy yaml.Node
	if yaml.Unmarshal(raw, &copy) != nil || len(copy.Content) != 1 {
		return yaml.Node{}, false
	}
	return *copy.Content[0], true
}
