// Package instructions reads and rewrites the instruction files (CLAUDE.md,
// AGENTS.md, GEMINI.md, ...) that targets load, and attaches shared
// instruction files (single-file extras) to them.
package instructions

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// AgentsFile is the cross-tool instruction file name.
const AgentsFile = "AGENTS.md"

// Read-order entry kinds.
const (
	KindMain     = "main"     // the target's own instruction file
	KindFallback = "fallback" // read only when the main file is absent (project mode)
	KindRules    = "rules"    // a directory of extra .md files loaded too
	KindUnread   = "unread"   // a user-level AGENTS.md the target never reads
)

// Entry is one file (or rules directory) in a target's read order.
type Entry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Exists bool   `json:"exists"`
	// Read reports whether the target loads it now.
	Read bool `json:"read"`
	// Count is the number of .md files in a rules directory.
	Count int `json:"count,omitempty"`
}

// Resolve returns the target's files as absolute paths: project-mode paths
// are joined to projectRoot.
func Resolve(it config.InstructionsTarget, projectRoot string) config.InstructionsTarget {
	abs := func(p string) string {
		if p == "" || projectRoot == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(projectRoot, p)
	}
	it.Path, it.Fallback, it.Rules = abs(it.Path), abs(it.Fallback), abs(it.Rules)
	return it
}

// ReadOrder lists the files a target loads, in the order it loads them. it
// must be resolved (absolute paths). global adds the user-level AGENTS.md a
// target with a project fallback does not read.
func ReadOrder(it config.InstructionsTarget, global bool) []Entry {
	mainExists := fileExists(it.Path)
	out := []Entry{{Path: it.Path, Kind: KindMain, Exists: mainExists, Read: mainExists}}
	if it.Fallback != "" {
		exists := fileExists(it.Fallback)
		out = append(out, Entry{Path: it.Fallback, Kind: KindFallback, Exists: exists, Read: exists && !mainExists})
	}
	if it.Rules != "" {
		n := countMarkdown(it.Rules)
		out = append(out, Entry{Path: it.Rules, Kind: KindRules, Exists: n > 0, Read: n > 0, Count: n})
	}
	if global && filepath.Base(it.Path) != AgentsFile && it.Import {
		out = append(out, Entry{Path: filepath.Join(filepath.Dir(it.Path), AgentsFile), Kind: KindUnread})
	}
	return out
}

// ImportLines returns the 1-based numbers of the lines that are @path
// imports: tool-specific syntax other tools read as plain text.
func ImportLines(content string) []int {
	var out []int
	fence := false
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fence = !fence
			continue
		}
		if !fence && isImport(trimmed) {
			out = append(out, i+1)
		}
	}
	return out
}

func isImport(trimmed string) bool {
	return len(trimmed) > 1 && trimmed[0] == '@' && !strings.ContainsAny(trimmed, " \t")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func countMarkdown(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			n++
		}
		return nil
	})
	return n
}

// Assignment is a shared instruction file attached to a target file.
type Assignment struct {
	Name   string `json:"name"`
	Mode   string `json:"mode"`
	Status string `json:"status"`
	// Reason explains a status that is not synced: ReasonFolderLink or
	// ReasonDirectory.
	Reason string `json:"reason,omitempty"`
}

// ReasonFolderLink: the target is a Windows directory junction to the file,
// which tools cannot read.
const ReasonFolderLink = "folder_link"

// ReasonDirectory: a real directory sits at the target path, so sync will not
// replace it.
const ReasonDirectory = "directory"

// Assignments returns the single-file extras whose targets write file, in
// config order.
func Assignments(extras []config.ExtraConfig, file string, r Resolver) []Assignment {
	out := []Assignment{}
	for _, extra := range extras {
		if extra.File == "" {
			continue
		}
		for j := range extra.Targets {
			f := ExtraFile(extra, j, r)
			if samePath(f.Target, file) {
				a := Assignment{Name: extra.Name, Mode: f.Mode}
				a.Status, a.Reason = FileStatus(f)
				out = append(out, a)
			}
		}
	}
	return out
}

// FileStatus returns the sync status of one target of a shared file and,
// when it is not synced, the reason (see Assignment).
func FileStatus(f syncpkg.ExtraFile) (string, string) {
	status := syncpkg.ExtraFileStatus(f)
	if status == "synced" {
		return status, ""
	}
	return status, notSyncedReason(f.Target)
}

// folderLink reports whether path is a directory junction rather than a
// symlink. Only Windows has them.
func notSyncedReason(path string) string {
	info, err := os.Lstat(path)
	switch {
	case err != nil || info.Mode()&os.ModeSymlink != 0:
		return ""
	case utils.IsSymlinkOrJunction(path):
		return ReasonFolderLink
	case info.IsDir():
		return ReasonDirectory
	}
	return ""
}

func samePath(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }
