package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// How a target reaches the project's AGENTS.md.
const (
	ReachDirect   = "direct"   // it reads AGENTS.md itself
	ReachFallback = "fallback" // it reads AGENTS.md because its own file is absent
	ReachImport   = "import"   // its own file has an @AGENTS.md line
	ReachLink     = "link"     // its own file is a symlink to AGENTS.md
	ReachShadowed = "shadowed" // its own file exists and hides AGENTS.md
	ReachMissing  = "missing"  // it reads only its own file, which is absent
)

// Shims that make a target read the project's AGENTS.md.
const (
	ShimImport = "import" // add an @AGENTS.md line to the target's own file
	ShimLink   = "link"   // create the target's own file as a symlink to AGENTS.md
)

// Reach reports whether a target reads the project's AGENTS.md.
type Reach struct {
	Target string `json:"target"`
	File   string `json:"file"` // the target's own file, relative to the project
	How    string `json:"how"`
	Reads  bool   `json:"reads"`
	Shim   string `json:"shim,omitempty"`
}

// ProjectReach reports how a target reaches <root>/AGENTS.md. it is the
// target's project-mode instructions (paths relative to root).
func ProjectReach(root, name string, it config.InstructionsTarget) Reach {
	r := Reach{Target: name, File: filepath.ToSlash(it.Path)}
	own := filepath.Join(root, it.Path)
	if filepath.Base(it.Path) == AgentsFile && filepath.Dir(it.Path) == "." {
		r.How, r.Reads = ReachDirect, true
		return r
	}
	info, err := os.Lstat(own)
	switch {
	case os.IsNotExist(err) && filepath.Base(it.Fallback) == AgentsFile:
		r.How, r.Reads = ReachFallback, true
	case os.IsNotExist(err):
		r.How, r.Shim = ReachMissing, ShimLink
		if it.Import {
			r.Shim = ShimImport
		}
	case err != nil:
		r.How = ReachShadowed
	case utils.IsLinkMode(own, info.Mode()) && pointsTo(own, filepath.Join(root, AgentsFile)):
		r.How, r.Reads = ReachLink, true
	case it.Import && importsAgents(own, filepath.Join(root, AgentsFile)):
		r.How, r.Reads = ReachImport, true
	default:
		r.How = ReachShadowed
		if it.Import {
			r.Shim = ShimImport
		}
	}
	return r
}

func pointsTo(link, target string) bool {
	dest, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(link), dest)
	}
	return samePath(dest, target)
}

// importsAgents reports whether the file at path has an @ line that resolves,
// relative to the file's directory, to agents.
func importsAgents(path, agents string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, n := range ImportLines(string(data)) {
		ref := strings.TrimPrefix(strings.TrimSpace(lines[n-1]), "@")
		if !filepath.IsAbs(ref) {
			ref = filepath.Join(filepath.Dir(path), ref)
		}
		if samePath(ref, agents) {
			return true
		}
	}
	return false
}

// ApplyShim makes a target read <root>/AGENTS.md: import adds an @AGENTS.md
// line at the top of its own file (backed up first), link creates its own
// file as a relative symlink.
func ApplyShim(root string, it config.InstructionsTarget, shim string) error {
	own := filepath.Join(root, it.Path)
	agents := filepath.Join(root, AgentsFile)
	rel, err := filepath.Rel(filepath.Dir(own), agents)
	if err != nil {
		return err
	}
	switch shim {
	case ShimImport:
		if !it.Import {
			return fmt.Errorf("%s does not follow @ imports", filepath.Base(own))
		}
		data, err := os.ReadFile(own)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			if importsAgents(own, agents) {
				return nil
			}
			if err := syncpkg.BackupFile(own, syncpkg.BackupReasonShim); err != nil {
				return err
			}
		}
		content := "@" + filepath.ToSlash(rel) + "\n"
		if len(data) > 0 {
			content += "\n" + string(data)
		}
		return WriteFile(own, content)
	case ShimLink:
		if _, err := os.Lstat(own); err == nil {
			return fmt.Errorf("%s already exists", own)
		}
		if err := os.MkdirAll(filepath.Dir(own), 0755); err != nil {
			return err
		}
		return os.Symlink(rel, own)
	default:
		return fmt.Errorf("unknown shim %q: must be import or link", shim)
	}
}
