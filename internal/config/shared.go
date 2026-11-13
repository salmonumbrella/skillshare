package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/sharedfile"
	"skillshare/internal/utils"
)

type SharedPointer struct {
	Path string              `yaml:"path"`
	Skip map[string][]string `yaml:"skip,omitempty"`
}

// UnmarshalYAML keeps the opt-in pointer strict while allowing future skip kinds.
func (p *SharedPointer) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("shared must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if seen[k.Value] {
			return fmt.Errorf("duplicate shared.%s", k.Value)
		}
		seen[k.Value] = true
		switch k.Value {
		case "path":
			if v.Kind != yaml.ScalarNode || v.Tag != "!!str" || strings.TrimSpace(v.Value) == "" || strings.ContainsRune(v.Value, 0) {
				return fmt.Errorf("shared.path must be a nonempty path")
			}
			p.Path = v.Value
		case "skip":
			if v.Kind != yaml.MappingNode {
				return fmt.Errorf("shared.skip must be a mapping")
			}
			p.Skip = map[string][]string{}
			for j := 0; j < len(v.Content); j += 2 {
				kind, patterns := v.Content[j], v.Content[j+1]
				switch kind.Value {
				case "extras", "hooks", "mcp", "plugins":
				default:
					return fmt.Errorf("unknown shared.skip.%s", kind.Value)
				}
				if _, ok := p.Skip[kind.Value]; ok {
					return fmt.Errorf("duplicate shared.skip.%s", kind.Value)
				}
				if patterns.Kind != yaml.SequenceNode {
					return fmt.Errorf("shared.skip.%s must be a list", kind.Value)
				}
				p.Skip[kind.Value] = []string{}
				for _, pattern := range patterns.Content {
					if pattern.Kind != yaml.ScalarNode || pattern.Tag != "!!str" || sharedfile.ValidSkipPattern(pattern.Value) != nil {
						return fmt.Errorf("invalid shared.skip.%s pattern", kind.Value)
					}
					p.Skip[kind.Value] = append(p.Skip[kind.Value], pattern.Value)
				}
			}
		default:
			return fmt.Errorf("unknown shared.%s", k.Value)
		}
	}
	if p.Path == "" {
		return fmt.Errorf("shared.path is required")
	}
	return nil
}

const (
	SharedOwned       = "shared_owned"
	SharedUnavailable = "shared_unavailable"
	SharedPublication = "shared_publication"
)

type SharedError struct {
	Code, Kind, Name, Path string
	Cause                  error
}

func (e *SharedError) Error() string {
	if e.Code == SharedOwned {
		return fmt.Sprintf("%s: %s %s comes from %s; edit the shared file, or skip or override it on this machine", e.Code, e.Kind, e.Name, e.Path)
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v; pull the fix, upgrade skillshare, or edit shared.path", e.Code, e.Cause)
	}
	return e.Code
}
func (e *SharedError) Unwrap() error { return e.Cause }

type SharedEntry struct {
	Name   string `json:"name"`
	Origin string `json:"origin"`
}
type SharedStatus struct {
	Enabled  bool                 `json:"enabled"`
	Usable   bool                 `json:"usable"`
	Path     string               `json:"path,omitempty"`
	Reason   sharedfile.Reason    `json:"reason,omitempty"`
	Findings []sharedfile.Finding `json:"findings,omitempty"`
	Entries  []SharedEntry        `json:"entries,omitempty"`
	Warnings []string             `json:"warnings,omitempty"`
}

type sharedLayer struct {
	document     *sharedfile.Document
	unavailable  *sharedfile.Unavailable
	status       SharedStatus
	origins      map[string]sharedfile.Origin
	baseline     map[string]ExtraConfig
	declarations map[string]yaml.Node
}

func (c *Config) SharedEnabled() bool { return c.Shared != nil }
func (c *Config) SharedStatus() SharedStatus {
	if !c.SharedEnabled() {
		return SharedStatus{}
	}
	if c.sharedLayer == nil {
		return SharedStatus{Enabled: true, Path: c.Shared.Path, Reason: sharedfile.Unreadable}
	}
	status := c.sharedLayer.status
	status.Findings = append([]sharedfile.Finding(nil), status.Findings...)
	status.Entries = append([]SharedEntry(nil), status.Entries...)
	status.Warnings = append([]string(nil), status.Warnings...)
	return status
}
func (c *Config) ExtraOrigin(name string) sharedfile.Origin {
	if c.sharedLayer != nil {
		if origin, ok := c.sharedLayer.origins[name]; ok {
			return origin
		}
	}
	return sharedfile.Local
}
func (c *Config) CheckExtraWritable(name string) error {
	if c.ExtraOrigin(name) == sharedfile.Shared {
		return &SharedError{Code: SharedOwned, Kind: "extras", Name: name, Path: c.sharedLayer.status.Path}
	}
	return nil
}
func (c *Config) CheckSharedApply() error {
	if !c.SharedEnabled() {
		return nil
	}
	if c.sharedLayer != nil && c.sharedLayer.status.Usable {
		return nil
	}
	var cause error
	if c.sharedLayer != nil {
		cause = c.sharedLayer.unavailable
	}
	if cause == nil {
		cause = &sharedfile.Unavailable{Path: c.Shared.Path, Reason: sharedfile.Unreadable}
	}
	return &SharedError{Code: SharedUnavailable, Path: c.SharedStatus().Path, Cause: cause}
}

func resolvedSharedPointer(configPath string, p *SharedPointer) string {
	if p == nil {
		return ""
	}
	path := sharedfile.ResolvePath(filepath.Dir(configPath), p.Path)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func (c *Config) loadShared(path string) error {
	if c.Shared == nil {
		return nil
	}
	resolved := resolvedSharedPointer(path, c.Shared)
	dest, err := utils.ConfigWritePath(path, false)
	if err != nil {
		return err
	}
	sharedDest, err := utils.ConfigWritePath(resolved, false)
	if err != nil {
		sharedDest = resolved
	}
	absDest, _ := filepath.Abs(dest)
	absShared, _ := filepath.Abs(sharedDest)
	if utils.PathsEqual(absDest, absShared) {
		return fmt.Errorf("shared.path must not point at config.yaml")
	}
	layer := &sharedLayer{status: SharedStatus{Enabled: true, Path: resolved}, baseline: map[string]ExtraConfig{}, declarations: map[string]yaml.Node{}}
	c.sharedLayer = layer
	doc, unavailable := sharedfile.Open(resolved)
	if unavailable == nil {
		var declared []ExtraConfig
		declared, unavailable = decodeSharedExtras(doc)
		if unavailable == nil {
			layer.document = doc
			resolvedExtras := resolveSharedExtras(doc.Dir, declared, &layer.status.Warnings)
			c.Extras, layer.origins = sharedfile.Merge(resolvedExtras, c.Extras, func(e ExtraConfig) string { return e.Name }, c.Shared.Skip["extras"])
			if section := doc.Section("extras"); section != nil {
				for i, extra := range declared {
					layer.declarations[extra.Name] = *section.Content[i]
				}
			}
			for _, extra := range c.Extras {
				if layer.origins[extra.Name] == sharedfile.Shared {
					layer.baseline[extra.Name] = cloneSharedExtra(extra)
				}
			}
			for _, extra := range declared {
				layer.status.Entries = append(layer.status.Entries, SharedEntry{Name: extra.Name, Origin: string(layer.origins[extra.Name])})
			}
			for _, extra := range c.Extras {
				if layer.origins[extra.Name] == sharedfile.Local {
					layer.status.Entries = append(layer.status.Entries, SharedEntry{Name: extra.Name, Origin: string(sharedfile.Local)})
				}
			}
			layer.status.Usable = true
			return nil
		}
	}
	layer.unavailable = unavailable
	layer.status.Reason = unavailable.Reason
	layer.status.Findings = unavailable.Findings
	return nil
}
