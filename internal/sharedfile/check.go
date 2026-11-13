package sharedfile

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/audit"
)

var credentialsURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s/@:]+:[^\s/@]+@`)
var parserLine = regexp.MustCompile(`line ([0-9]+)`)

// Check validates the exact publication bytes. Privacy takes precedence over
// structural diagnostics. Parser messages are reduced to locations only.
func Check(path string, data []byte) (*Document, *Unavailable) {
	fail := func(reason Reason, findings ...Finding) (*Document, *Unavailable) {
		return nil, &Unavailable{Path: path, Reason: reason, Findings: findings}
	}
	if len(data) > maxBytes {
		return fail(Unreadable)
	}
	findings, err := secretFindings(data)
	if err != nil {
		return fail(Privacy, Finding{Rule: "secret-rules-unavailable"})
	}
	for _, loc := range credentialsURL.FindAllIndex(data, -1) {
		findings = append(findings, Finding{Line: bytes.Count(data[:loc[0]], []byte("\n")) + 1, Rule: "url-credentials"})
	}
	if len(findings) > 0 {
		return fail(Privacy, findings...)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		f := Finding{}
		if match := parserLine.FindStringSubmatch(err.Error()); len(match) > 1 {
			f.Line, _ = strconv.Atoi(match[1])
		}
		return fail(Parse, f)
	}
	if dec.Decode(new(yaml.Node)) != io.EOF || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fail(Parse)
	}
	root := doc.Content[0]
	// Check assignments before rejecting structure, including future/unknown keys.
	var structure []Finding
	var visit func(*yaml.Node)
	visit = func(n *yaml.Node) {
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			structure = append(structure, Finding{Line: n.Line, Column: n.Column})
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Kind != yaml.ScalarNode || seen[k.Value] {
					structure = append(structure, Finding{Line: k.Line, Column: k.Column})
				}
				seen[k.Value] = true
				if v.Kind == yaml.ScalarNode {
					matches, e := secretFindings([]byte(k.Value + ": " + strconv.Quote(v.Value)))
					if e != nil {
						findings = append(findings, Finding{Line: k.Line, Rule: "secret-rules-unavailable"})
					}
					for _, m := range matches {
						m.Line = k.Line
						m.Column = k.Column
						findings = append(findings, m)
					}
				}
			}
		}
		for _, child := range n.Content {
			visit(child)
		}
	}
	visit(root)
	if len(findings) > 0 {
		return fail(Privacy, findings...)
	}
	if len(structure) > 0 {
		return fail(Parse, structure...)
	}
	d := document(path, data, root)
	version := d.Section("version")
	if version == nil || version.Kind != yaml.ScalarNode || version.Tag != "!!int" || version.Value != "1" {
		return fail(UnsupportedVersion)
	}
	later := " hooks mcp plugins mode target_naming ignore audit hub gitlab_hosts azure_hosts cnb_hosts gitea_hosts context_budget "
	machine := " targets sources projects shared git_root source agents_source extras_source config_dir cli "
	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		if k.Value == "version" || k.Value == "extras" {
			continue
		}
		reason := UnknownKey
		if strings.Contains(later, " "+k.Value+" ") {
			reason = NewerSection
		}
		if strings.Contains(machine, " "+k.Value+" ") {
			reason = MachineKey
		}
		return fail(reason, Finding{Line: k.Line, Column: k.Column, Key: k.Value})
	}
	if extras := d.Section("extras"); extras != nil {
		if extras.Kind != yaml.SequenceNode {
			return fail(Entry, Finding{Line: extras.Line})
		}
		for _, extra := range extras.Content {
			if f := unknownField(extra, " name source file targets "); f != nil {
				return fail(f.reason, f.finding)
			}
			for i := 0; i < len(extra.Content); i += 2 {
				if extra.Content[i].Value == "targets" {
					targets := extra.Content[i+1]
					if targets.Kind != yaml.SequenceNode {
						return fail(Entry, Finding{Line: targets.Line})
					}
					for _, target := range targets.Content {
						if f := unknownField(target, " path mode flatten extension as include exclude "); f != nil {
							return fail(f.reason, f.finding)
						}
					}
				}
			}
		}
	}
	return d, nil
}

type fieldFailure struct {
	reason  Reason
	finding Finding
}

func unknownField(n *yaml.Node, allowed string) *fieldFailure {
	if n.Kind != yaml.MappingNode {
		return &fieldFailure{Entry, Finding{Line: n.Line}}
	}
	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if !strings.Contains(allowed, " "+k.Value+" ") {
			return &fieldFailure{UnknownKey, Finding{Line: k.Line, Column: k.Column, Key: k.Value}}
		}
		valid := v.Kind == yaml.ScalarNode && v.Tag == "!!str"
		switch k.Value {
		case "targets":
			valid = v.Kind == yaml.SequenceNode
		case "flatten":
			valid = v.Kind == yaml.ScalarNode && v.Tag == "!!bool"
		case "include", "exclude":
			valid = v.Kind == yaml.SequenceNode
			for _, item := range v.Content {
				valid = valid && item.Kind == yaml.ScalarNode && item.Tag == "!!str"
			}
		}
		if !valid {
			return &fieldFailure{Entry, Finding{Line: v.Line, Column: v.Column, Key: k.Value}}
		}
	}
	return nil
}

func secretFindings(data []byte) ([]Finding, error) {
	matches, err := audit.SecretMatches(data)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, m := range matches {
		findings = append(findings, Finding{Line: m.Line, Rule: m.RuleID})
	}
	return findings, nil
}
