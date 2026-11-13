package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	syncpkg "skillshare/internal/sync"
)

// Convert methods.
const (
	MethodImport = "import" // move the content to AGENTS.md; the file keeps an @AGENTS.md line
	MethodRename = "rename" // rename the file to AGENTS.md (project mode only)
	MethodCopy   = "copy"   // copy the content to AGENTS.md; both files stay
)

// Change statuses.
const (
	ChangeNew      = "new"
	ChangeModified = "modified"
	ChangeRemoved  = "removed"
)

// Change is one file a conversion writes or removes.
type Change struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// ConvertOptions controls PlanConvert.
type ConvertOptions struct {
	Method string
	// KeepToolLines leaves the @import lines in the original file instead of
	// moving them to AGENTS.md, where other tools would read them as text.
	KeepToolLines bool
	// Project allows rename: without a CLAUDE.md, a project's AGENTS.md is read
	// instead, but a user-level AGENTS.md never is.
	Project bool
	// Dest is where the AGENTS.md content goes; default is next to the file.
	Dest string
	// Managed writes the @Dest line as a managed import block (a shared
	// instruction file attached in import mode) instead of a plain line.
	Managed bool
	// Append adds the moved content to the end of an existing Dest (a shared
	// instruction file) instead of creating it.
	Append bool
}

// PlanConvert computes the changes that turn src (e.g. CLAUDE.md) into an
// AGENTS.md other tools read. It writes nothing. skillshare's managed import
// block always stays in src.
func PlanConvert(src string, opts ConvertOptions) ([]Change, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s does not exist", src)
		}
		return nil, err
	}
	content := string(data)
	dest := opts.Dest
	if dest == "" {
		dest = filepath.Join(filepath.Dir(src), AgentsFile)
	}
	var existing []byte
	if opts.Append {
		if existing, err = os.ReadFile(dest); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else if _, err := os.Lstat(dest); err == nil {
		return nil, fmt.Errorf("%s already exists", dest)
	}

	// Only import keeps the original file to leave @import lines in.
	kept, body := splitToolLines(content, opts.KeepToolLines && opts.Method == MethodImport)
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("%s has nothing to move to %s", filepath.Base(src), AgentsFile)
	}

	switch opts.Method {
	case MethodImport:
		after := kept
		if opts.Managed {
			after, _ = syncpkg.AddManagedImport(kept, "@"+dest)
		} else {
			line := "@" + dest
			if filepath.Dir(dest) == filepath.Dir(src) {
				line = "@" + filepath.Base(dest)
			}
			after = line + "\n"
			if kept != "" {
				after += "\n" + kept
			}
		}
		return []Change{
			destChange(dest, existing, body, opts.Append),
			{Path: src, Status: ChangeModified, Before: content, After: after},
		}, nil
	case MethodRename:
		if !opts.Project {
			return nil, fmt.Errorf("rename is only possible in a project: the tool does not read a user-level %s", AgentsFile)
		}
		// Removing src would drop the managed block, and the next sync would
		// recreate src holding only that block, hiding AGENTS.md again.
		if kept != "" {
			return nil, fmt.Errorf("%s uses a shared instruction file, so renaming it would stop %s being read", filepath.Base(src), AgentsFile)
		}
		local := strings.TrimSuffix(src, filepath.Ext(src)) + ".local" + filepath.Ext(src)
		if _, err := os.Stat(local); err == nil {
			return nil, fmt.Errorf("%s exists, so %s would still not be read", filepath.Base(local), AgentsFile)
		}
		return []Change{
			{Path: dest, Status: ChangeNew, After: body},
			{Path: src, Status: ChangeRemoved, Before: content},
		}, nil
	case MethodCopy:
		return []Change{{Path: dest, Status: ChangeNew, After: body}}, nil
	default:
		return nil, fmt.Errorf("unknown method %q: must be import, rename, or copy", opts.Method)
	}
}

// destChange creates dest with body, or with Append puts body after the
// existing content, separated by one blank line.
func destChange(dest string, existing []byte, body string, appendTo bool) Change {
	if !appendTo || existing == nil {
		return Change{Path: dest, Status: ChangeNew, After: body}
	}
	before := string(existing)
	after := body
	if head := strings.TrimRight(before, "\n"); strings.TrimSpace(head) != "" {
		after = head + "\n\n" + body
	}
	return Change{Path: dest, Status: ChangeModified, Before: before, After: after}
}

// HasMovableContent reports whether content still has instructions to move to
// an AGENTS.md once skillshare's managed block and the @import lines stay put.
// A file that is only those lines has nothing to convert.
func HasMovableContent(content string) bool {
	_, body := splitToolLines(content, true)
	return strings.TrimSpace(body) != ""
}

// splitToolLines separates content into the lines that stay in the original
// file (the managed import block, plus @import lines when keepImports) and
// the rest. Both parts are trimmed of surrounding blank lines and end with a
// newline when non-empty.
func splitToolLines(content string, keepImports bool) (kept, body string) {
	lines := strings.Split(content, "\n")
	stay := map[int]bool{}
	for _, i := range syncpkg.ManagedImportLines(content) {
		stay[i] = true
	}
	if keepImports {
		for _, n := range ImportLines(content) {
			stay[n-1] = true
		}
	}
	var k, b []string
	for i, l := range lines {
		if stay[i] {
			k = append(k, l)
		} else {
			b = append(b, l)
		}
	}
	return tidy(k), tidy(b)
}

func tidy(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	// Collapse the blank runs left where lines were taken out.
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) == "" && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// Apply writes changes, backing up every existing file it rewrites or removes
// first. It refuses a new file that appeared since planning.
func Apply(changes []Change) error {
	for _, c := range changes {
		if c.Status == ChangeNew {
			if _, err := os.Lstat(c.Path); err == nil {
				return fmt.Errorf("%s already exists", c.Path)
			}
		}
	}
	// Back up everything before touching anything.
	for _, c := range changes {
		if c.Status != ChangeNew {
			if err := syncpkg.BackupFile(c.Path, syncpkg.BackupReasonConvert); err != nil {
				return err
			}
		}
	}
	// New files first, so a failure never leaves the content only in a backup.
	order := slices.Clone(changes)
	slices.SortStableFunc(order, func(a, b Change) int {
		return rank(a.Status) - rank(b.Status)
	})
	for _, c := range order {
		switch c.Status {
		case ChangeRemoved:
			if err := os.Remove(c.Path); err != nil {
				return fmt.Errorf("remove %s: %w", c.Path, err)
			}
		default:
			if err := WriteFile(c.Path, c.After); err != nil {
				return err
			}
		}
	}
	return nil
}

func rank(status string) int {
	switch status {
	case ChangeNew:
		return 0
	case ChangeModified:
		return 1
	default:
		return 2
	}
}

// WriteFile writes content to path, creating parent directories and keeping
// an existing file's permissions. It writes through a symlink.
func WriteFile(path, content string) error {
	perm := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
