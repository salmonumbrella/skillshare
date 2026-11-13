package config

import (
	"os"
	"path/filepath"
	"sort"
)

// InstructionsRider is a built-in target that is not configured but reads
// the skills a configured target syncs, from its own skills folder or one it
// also scans, while keeping an instruction file of its own. Codex reads
// ~/.agents/skills (universal) but ~/.codex/AGENTS.md, not ~/.agents/AGENTS.md.
type InstructionsRider struct {
	Name string
	// Via is the configured target whose skills folder the rider reads.
	Via string
	InstructionsTarget
}

// InstructionRiders returns the riders of a configured target in global mode,
// sorted by name. A rider uses the target's folder as its own skills folder
// (Codex) or also scans it (Gemini, Pi). Only tools found on this machine
// count (see specInstalled). A tool whose file is the target's own is a
// reader (InstructionReaders), not a rider; one whose file belongs to another
// configured target (Antigravity's GEMINI.md, with gemini configured) is
// managed on that target's page. Riders are listed on universal's page only:
// another target whose folder happens to be ~/.agents/skills (codex, with
// universal not configured) does not collect every tool reading it.
func InstructionRiders(target string, targets map[string]TargetConfig) []InstructionsRider {
	tc, ok := targets[target]
	if !ok || tc.ProjectRoot() != "" || target != "universal" {
		return nil
	}
	skills := tc.SkillsConfig().Path
	if skills == "" {
		return nil
	}
	skills = filepath.Clean(expandPath(skills))
	owned := map[string]bool{}
	for name, other := range targets {
		if it, ok := TargetInstructions(name, other, false); ok {
			owned[filepath.Clean(it.Path)] = true
		}
	}

	specs, err := loadTargetSpecs()
	if err != nil {
		return nil
	}
	var riders []InstructionsRider
	for _, spec := range specs {
		if spec.Name == "" || spec.Name == target || specConfigured(spec, targets) || !specReadsSkills(spec, skills) {
			continue
		}
		it, ok := specInstructions(spec, false)
		if !ok || owned[filepath.Clean(it.Path)] || !specInstalled(spec, skills, it.Path) {
			continue
		}
		riders = append(riders, InstructionsRider{Name: spec.Name, Via: target, InstructionsTarget: it})
	}
	// Tools sharing one file (Antigravity reads Gemini's GEMINI.md) are one entry,
	// under the tool that owns the file rather than the one declaring same_as.
	sort.SliceStable(riders, func(i, j int) bool {
		if (riders[i].SameAs == "") != (riders[j].SameAs == "") {
			return riders[i].SameAs == ""
		}
		return riders[i].Name < riders[j].Name
	})
	seen := map[string]bool{}
	unique := riders[:0]
	for _, r := range riders {
		if key := filepath.Clean(r.Path); !seen[key] {
			seen[key] = true
			unique = append(unique, r)
		}
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].Name < unique[j].Name })
	return unique
}

// InstructionReaders names the built-in targets that read path as their
// global instruction file because they declare same_as target (Cline and Warp
// read universal's ~/.agents/AGENTS.md), sorted. They are listed whether or
// not they are installed: the list says who reads the file, and some of these
// tools keep no folder skillshare could detect them by.
func InstructionReaders(target, path string) []string {
	specs, err := loadTargetSpecs()
	if err != nil || path == "" {
		return nil
	}
	var names []string
	for _, spec := range specs {
		if spec.Name == "" || spec.Name == target || spec.Instructions.SameAs != target {
			continue
		}
		it, ok := specInstructions(spec, false)
		if ok && filepath.Clean(it.Path) == filepath.Clean(path) {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	return names
}

// specReadsSkills reports whether a tool reads skills from folder, as its own
// skills folder or one it also scans.
func specReadsSkills(spec targetSpec, folder string) bool {
	if spec.Skills.Global != "" && filepath.Clean(normalizeTargetPath(spec.Skills.Global)) == folder {
		return true
	}
	for _, p := range spec.AlsoScans.Global {
		if filepath.Clean(normalizeTargetPath(p)) == folder {
			return true
		}
	}
	return false
}

// specInstalled guesses whether a tool is on this machine, like init does: its
// detect folder, else the folder above its own skills folder (~/.gemini). A
// tool whose own skills folder is the shared one (Zed) has nothing of its own
// there, so the folder of its instruction file (~/.config/zed) stands in.
func specInstalled(spec targetSpec, shared, instructions string) bool {
	dir := filepath.Dir(instructions)
	if own := filepath.Clean(normalizeTargetPath(spec.Skills.Global)); spec.Skills.Global != "" && own != shared {
		dir = filepath.Dir(own)
	}
	if spec.Detect != "" {
		dir = normalizeTargetPath(spec.Detect)
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// specConfigured reports whether a spec is configured under its name or an alias.
func specConfigured(spec targetSpec, targets map[string]TargetConfig) bool {
	if _, ok := targets[spec.Name]; ok {
		return true
	}
	for _, alias := range spec.Aliases {
		if _, ok := targets[alias]; ok {
			return true
		}
	}
	return false
}
