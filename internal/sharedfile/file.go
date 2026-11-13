// Package sharedfile checks portable declarations without depending on config.
package sharedfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const Version = 1
const DefaultName = "config.shared.yaml"
const maxBytes = 1 << 20

type Reason string

const (
	Missing            Reason = "missing"
	Unreadable         Reason = "unreadable"
	Parse              Reason = "parse"
	UnsupportedVersion Reason = "version"
	NewerSection       Reason = "newer-section"
	MachineKey         Reason = "machine-key"
	UnknownKey         Reason = "unknown-key"
	Privacy            Reason = "privacy"
	Entry              Reason = "entry"
)

// Finding holds locations and identifiers, never values or parser messages.
type Finding struct {
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Entry  string `json:"entry,omitempty"`
	Key    string `json:"key,omitempty"`
	Rule   string `json:"rule,omitempty"`
}

type Unavailable struct {
	Path     string
	Reason   Reason
	Findings []Finding
}

func (u *Unavailable) Error() string {
	message := fmt.Sprintf("shared settings %s: %s", u.Path, u.Reason)
	if len(u.Findings) > 0 {
		f := u.Findings[0]
		if f.Key != "" {
			message += fmt.Sprintf(" (key %s)", f.Key)
		}
		if f.Rule != "" {
			message += fmt.Sprintf(" (rule %s)", f.Rule)
		}
		if f.Line > 0 {
			message += fmt.Sprintf(" at line %d", f.Line)
		}
	}
	return message
}

type Document struct {
	Path, Dir string
	Raw       []byte
	Root      *yaml.Node
}

func (d *Document) Section(key string) *yaml.Node {
	for i := 0; i+1 < len(d.Root.Content); i += 2 {
		if d.Root.Content[i].Value == key {
			return d.Root.Content[i+1]
		}
	}
	return nil
}

// Open bounds reads even for non-regular files and drops filesystem error text.
func Open(path string) (*Document, *Unavailable) {
	f, err := os.Open(path)
	if err != nil {
		reason := Unreadable
		if os.IsNotExist(err) {
			reason = Missing
		}
		return nil, &Unavailable{Path: path, Reason: reason}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, &Unavailable{Path: path, Reason: Unreadable}
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil || len(raw) > maxBytes {
		return nil, &Unavailable{Path: path, Reason: Unreadable}
	}
	return Check(path, raw)
}

func document(path string, raw []byte, root *yaml.Node) *Document {
	return &Document{Path: path, Dir: filepath.Dir(path), Raw: append([]byte(nil), raw...), Root: root}
}
