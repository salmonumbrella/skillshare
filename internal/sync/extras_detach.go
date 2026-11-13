package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sharedfile"
	"skillshare/internal/utils"
)

type DetachResult struct {
	Name, Target string
	Removed      int
	Kept, Errors []string
}

type SharedExtrasRun struct {
	cfg       *config.Config
	state     *config.SharedStateFile
	dryRun    bool
	release   func()
	failed    error
	protected map[string]bool
	recovered map[string]bool
	// The recorded declaration's relative paths precede this run's pointer update.
	declarationDir string
}

type SharedExtrasScope struct{ Name, Target string }

func BeginSharedExtras(cfg *config.Config, dryRun bool, scopes ...SharedExtrasScope) (*SharedExtrasRun, error) {
	if !cfg.SharedEnabled() {
		return nil, nil
	}
	path := cfg.SharedStatus().Path
	var state *config.SharedStateFile
	var release func()
	var err error
	if dryRun {
		state, err = config.LoadSharedState(config.ConfigPath(), cfg.EffectiveGitRoot(), filepath.Dir(path))
	} else {
		state, release, err = config.BeginSharedStateWrite(context.Background(), config.ConfigPath(), cfg.EffectiveGitRoot(), filepath.Dir(path))
		if err == nil {
			fresh, loadErr := config.LoadReadOnlyPath(config.ConfigPath())
			if loadErr != nil {
				err = loadErr
			} else if !fresh.SharedEnabled() || !utils.PathsEqual(fresh.SharedStatus().Path, path) || !utils.PathsEqual(fresh.EffectiveGitRoot(), cfg.EffectiveGitRoot()) {
				err = fmt.Errorf("shared settings identity changed; retry the sync")
			} else {
				cfg = fresh
			}
		}
	}
	if err == nil {
		err = cfg.CheckSharedApply()
	}
	if err != nil {
		if release != nil {
			release()
		}
		return nil, &config.SharedError{Code: config.SharedUnavailable, Path: path, Cause: err}
	}
	previousPath := state.State.Pointer
	if previousPath == "" {
		previousPath = path
	}
	r := &SharedExtrasRun{cfg: cfg, state: state, release: release, dryRun: dryRun, protected: map[string]bool{}, recovered: map[string]bool{}, declarationDir: filepath.Dir(previousPath)}
	if !dryRun && state.State.Pending != nil {
		p := state.State.Pending
		if len(scopes) > 0 && (scopes[0].Name != "" && scopes[0].Name != p.Placement.Name || scopes[0].Target != "" && !utils.PathsEqual(scopes[0].Target, p.Placement.Receipt.Targets[0].Path)) {
			err = fmt.Errorf("pending shared placement for %s at %s is outside the selected scope", p.Placement.Name, p.Placement.Receipt.Targets[0].Path)
		} else {
			err = r.recoverPending()
		}
		if err != nil {
			r.Close()
			return nil, &config.SharedError{Code: config.SharedUnavailable, Path: path, Cause: err}
		}
	}
	state.RecordPointer(path)
	return r, nil
}

func (r *SharedExtrasRun) Close() {
	if r != nil && r.release != nil {
		r.release()
	}
}
func (r *SharedExtrasRun) Config() *config.Config {
	if r == nil {
		return nil
	}
	return r.cfg
}
func (r *SharedExtrasRun) latch(err error) error {
	if err != nil && r.failed == nil {
		r.failed = &config.SharedError{Code: config.SharedUnavailable, Path: r.cfg.SharedStatus().Path, Cause: err}
	}
	return r.failed
}

func (r *SharedExtrasRun) Protected() map[string]bool {
	if r == nil {
		return nil
	}
	return r.protected
}
func (r *SharedExtrasRun) Save() error {
	if r == nil || r.dryRun {
		return nil
	}
	if r.failed != nil {
		return r.failed
	}
	return r.latch(r.state.Save())
}

func placedOutput(file string, target config.SharedPlacedTarget) string {
	if file == "" {
		return filepath.Clean(target.Path)
	}
	as := target.As
	if as == "" {
		as = file
	}
	return filepath.Join(target.Path, as)
}

// protectedOutput includes ancestors: replacing a directory must not erase a
// protected child, nor may a writer traverse a protected directory or link.
func protectedOutput(path string, protected map[string]bool) bool {
	path = filepath.Clean(path)
	for kept, yes := range protected {
		if !yes {
			continue
		}
		kept = filepath.Clean(kept)
		if utils.PathsEqual(path, kept) || utils.PathHasPrefix(path, kept+string(filepath.Separator)) || utils.PathHasPrefix(kept, path+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
func keptExtraResult() *ExtraResult {
	return &ExtraResult{Skipped: 1, Warnings: []string{"kept: edited by you"}}
}

func (r *SharedExtrasRun) currentTarget(old config.SharedPlacement, t config.SharedPlacedTarget, resolve func(string) string) (bool, bool) {
	var declaration config.ExtraConfig
	if err := old.Declaration.Decode(&declaration); err != nil {
		return false, false
	}
	var previous *config.ExtraTargetConfig
	for i := range declaration.Targets {
		candidate := &declaration.Targets[i]
		if utils.PathsEqual(sharedfile.ResolvePath(declarationDir(old, r.declarationDir), candidate.Path), t.Path) && candidate.As == t.As {
			previous = candidate
			break
		}
	}
	if previous == nil {
		return false, false
	}
	extension := previous.Extension
	if strings.ContainsAny(extension, `/\`) || strings.HasPrefix(extension, ".") {
		extension = sharedfile.ResolvePath(declarationDir(old, r.declarationDir), extension)
	}
	for _, extra := range r.cfg.Extras {
		if extra.Name != old.Name {
			continue
		}
		source := config.ResolveExtrasSourceDir(extra, r.cfg.EffectiveExtrasSource(), r.cfg.EffectiveSkillsSource())
		for _, now := range extra.Targets {
			if !utils.PathsEqual(resolve(now.Path), t.Path) || now.As != t.As {
				continue
			}
			mode := ExtraTargetMode(now.Mode, extra.File != "")
			if now.Extension != "" {
				mode = "copy"
			}
			same := mode == t.Mode && extra.File == old.Receipt.File && utils.PathsEqual(source, old.Receipt.Source) &&
				now.Extension == extension && now.Flatten == previous.Flatten &&
				slices.Equal(now.Include, previous.Include) && slices.Equal(now.Exclude, previous.Exclude)
			return same, r.cfg.ExtraOrigin(extra.Name) == sharedfile.Shared
		}
	}
	return false, false
}

func (r *SharedExtrasRun) Detach(resolve func(string) string, names ...string) []DetachResult {
	return r.detach(resolve, func(name string, _ config.SharedPlacedTarget) bool {
		return len(names) == 0 || slices.Contains(names, name)
	})
}

// DetachTarget keeps a TUI target-only action from touching other placements.
func (r *SharedExtrasRun) DetachTarget(resolve func(string) string, name, path string) []DetachResult {
	return r.detach(resolve, func(candidate string, target config.SharedPlacedTarget) bool {
		return candidate == name && utils.PathsEqual(target.Path, resolve(path))
	})
}

func (r *SharedExtrasRun) detach(resolve func(string) string, selected func(string, config.SharedPlacedTarget) bool) []DetachResult {
	if r == nil {
		return nil
	}
	if r.failed != nil {
		return []DetachResult{{Errors: []string{r.failed.Error()}}}
	}
	var results []DetachResult
	var remaining []config.SharedPlacement
	for _, old := range r.state.State.Extras {
		keep := old
		keep.Receipt.Targets = nil
		for _, target := range old.Receipt.Targets {
			if !selected(old.Name, target) {
				keep.Receipt.Targets = append(keep.Receipt.Targets, target)
				continue
			}
			if same, shared := r.currentTarget(old, target, resolve); same {
				if shared {
					keep.Receipt.Targets = append(keep.Receipt.Targets, target)
				}
				continue
			}
			result := DetachResult{Name: old.Name, Target: target.Path}
			if r.dryRun {
				results = append(results, result)
				keep.Receipt.Targets = append(keep.Receipt.Targets, target)
				continue
			}
			result = r.detachTarget(old, target)
			for _, path := range result.Kept {
				r.protected[filepath.Clean(path)] = true
			}
			if len(result.Errors) > 0 {
				r.protected[placedOutput(old.Receipt.File, target)] = true
				keep.Receipt.Targets = append(keep.Receipt.Targets, target)
			}
			results = append(results, result)
		}
		if len(keep.Receipt.Targets) > 0 {
			remaining = append(remaining, keep)
		}
	}
	r.state.State.Extras = remaining
	if err := r.Save(); err != nil {
		results = append(results, DetachResult{Errors: []string{err.Error()}})
	}
	return results
}

func (r *SharedExtrasRun) detachTarget(old config.SharedPlacement, target config.SharedPlacedTarget) DetachResult {
	result := DetachResult{Name: old.Name, Target: target.Path}
	if old.Receipt.File != "" {
		f := NewExtraFile(old.Receipt.Source, old.Receipt.File, target.Path, target.As, target.Mode)
		if parent, err := linkedPlacementParent(target.Path, placedOutput(old.Receipt.File, target)); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result
		} else if parent != "" {
			ForgetExtraTarget(f)
			result.Kept = append(result.Kept, parent)
			return result
		}
		if target.Native != nil {
			p := &config.SharedExtraIntent{Effects: []config.SharedExtraEffect{{Path: f.Target}}, MetadataAfter: target.Native}
			facts, err := sharedNativeFacts(p)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				return result
			}
			for name, expected := range target.Native {
				if facts[name] != expected {
					result.Errors = append(result.Errors, "native restore metadata changed; kept for recovery")
					return result
				}
			}
		}
		changed, kept, err := DetachExtraFileTarget(f)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
		if changed {
			result.Removed++
		}
		if kept {
			result.Kept = append(result.Kept, f.Target)
		}
		return result
	}
	switch target.Mode {
	case "copy", "merge", "":
		if info, err := os.Lstat(target.Path); err == nil && utils.IsLinkMode(target.Path, info.Mode()) {
			result.Kept = append(result.Kept, target.Path)
			return result
		}
		copies, err := loadSharedCopyTracker(target.Path)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result
		}
		for rel, expected := range target.Files {
			path := filepath.Join(target.Path, filepath.FromSlash(rel))
			// Receipts are private but still never authorize traversing outside a target.
			relative, err := filepath.Rel(target.Path, path)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				result.Errors = append(result.Errors, "invalid placement output path")
				continue
			}
			if parent, err := linkedPlacementParent(target.Path, path); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			} else if parent != "" {
				result.Kept = append(result.Kept, parent)
				copies.forget(rel)
				continue
			}
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				copies.forget(rel)
				continue
			}
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			if !info.Mode().IsRegular() && !utils.IsLinkMode(path, info.Mode()) {
				result.Kept = append(result.Kept, path)
				copies.forget(rel)
				continue
			}
			hash, err := sharedOutputIdentity(path)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			if hash != expected {
				result.Kept = append(result.Kept, path)
				copies.forget(rel)
				continue
			}
			if err := os.Remove(path); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			copies.forget(rel)
			result.Removed++
			cleanEmptyParents(filepath.Dir(path), target.Path)
		}
		if err := copies.save(); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	case "symlink":
		removed, kept, errs := ClearExtraTarget("", old.Receipt.Source, target.Path, "", target.Mode, nil)
		result.Removed = removed
		result.Errors = errs
		if kept {
			result.Kept = append(result.Kept, target.Path)
		}

	}
	return result
}

// A replaced directory link changes the referent of every output below it.
// Never follow it to hash/delete native content, or let replacement apply use it.
func linkedPlacementParent(base, output string) (string, error) {
	for parent := filepath.Dir(output); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && utils.IsLinkMode(parent, info.Mode()) {
			return parent, nil
		}
		if utils.PathsEqual(parent, filepath.Clean(base)) || filepath.Dir(parent) == parent {
			return "", nil
		}
	}
}

// Link identity remains available after its source disappears. A link replaced
// by a file or another link never inherits the old output's ownership.
func sharedOutputIdentity(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if utils.IsLinkMode(path, info.Mode()) {
		dest, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		return "link:" + filepath.Clean(resolveReadlink(dest, path)), nil
	}
	return utils.FileHash(path)
}

func declarationDir(p config.SharedPlacement, fallback string) string {
	if p.DeclarationDir != "" {
		return p.DeclarationDir
	}
	return fallback
}

// mergePlacement changes only the output whose native write was verified.
func (r *SharedExtrasRun) mergePlacement(next config.SharedPlacement) {
	target := &next.Receipt.Targets[0]
	var remaining []config.SharedPlacement
	for _, old := range r.state.State.Extras {
		if old.Name == next.Name && old.Receipt.Source == next.Receipt.Source && old.Receipt.File == next.Receipt.File {
			old.Receipt.Targets = slices.DeleteFunc(slices.Clone(old.Receipt.Targets), func(previous config.SharedPlacedTarget) bool {
				if previous.Path != target.Path || previous.As != target.As {
					return false
				}
				if target.Files != nil {
					for rel, hash := range previous.Files {
						if _, ok := target.Files[rel]; !ok {
							target.Files[rel] = hash
						}
					}
				}
				return true
			})
		}
		if len(old.Receipt.Targets) > 0 {
			remaining = append(remaining, old)
		}
	}
	r.state.State.Extras = append(remaining, next)
}

type placementChange struct {
	path, before, after string
	effects             []config.SharedExtraEffect
	metadata            map[string]string
	retainOnly          bool
	acquireOnly         bool
}
type placementWriter func(placementChange, func() error) error

func placementLinkAfter(place placementWriter, source string) string {
	if place == nil {
		return ""
	}
	return "link-to:" + evalOrClean(source)
}

func placeExtra(place placementWriter, change placementChange, mutate func() error) error {
	if place == nil {
		return mutate()
	}
	return place(change, mutate)
}

func (r *SharedExtrasRun) writer(extra config.ExtraConfig, source string, target config.SharedPlacedTarget) placementWriter {
	if r == nil || r.dryRun || r.cfg.ExtraOrigin(extra.Name) != sharedfile.Shared {
		return nil
	}
	return func(change placementChange, mutate func() error) error {
		if r.failed != nil {
			return r.failed
		}
		if change.acquireOnly {
			for _, old := range r.state.State.Extras {
				if old.Name != extra.Name || old.Receipt.Source != source || old.Receipt.File != extra.File {
					continue
				}
				for _, previous := range old.Receipt.Targets {
					if previous.Path == target.Path && previous.As == target.As {
						for name, expected := range previous.Native {
							if change.metadata[name] != expected {
								return r.latch(fmt.Errorf("native restore proof changed"))
							}
						}
						return nil
					}
				}
			}
			attach := 0
			for _, name := range []string{attachCreated, attachRestore, attachRestoreLink, attachRestoreJunction} {
				if change.metadata[name] != "absent" {
					attach++
				}
			}
			if attach == 0 && target.Mode == "copy" && !strings.HasPrefix(change.before, "link:") {
				return nil // identical unowned regular files remain user-owned
			}
			written := extraContentFingerprint([]byte(strings.TrimPrefix(change.before, "file:")))
			if attach != 1 || change.metadata[attachCreated] != "absent" && change.metadata[attachCreated] != extraContentFingerprint(nil) || target.Mode == "copy" && change.metadata[extraWritten] != written {
				return r.latch(fmt.Errorf("shared ownership requires intact native restore proof; kept %s", change.path))
			}
		}

		declaration, ok := r.cfg.SharedExtraDeclaration(extra.Name)
		if !ok {
			return r.latch(fmt.Errorf("shared declaration disappeared"))
		}
		rel, err := filepath.Rel(target.Path, change.path)
		if err != nil {
			return r.latch(err)
		}
		if extra.File != "" || target.Mode == "symlink" {
			rel = ""
		}
		before, err := placementFingerprint(change.path)
		if err != nil {
			return r.latch(err)
		}
		if change.before != "" && before != change.before {
			return r.latch(fmt.Errorf("shared destination changed before placement: %s", change.path))
		}
		p := &config.SharedExtraIntent{Placement: config.SharedPlacement{Name: extra.Name, Declaration: declaration, DeclarationDir: filepath.Dir(r.cfg.SharedStatus().Path), Receipt: config.SharedReceipt{Source: source, File: extra.File, Targets: []config.SharedPlacedTarget{target}}}, Relative: filepath.ToSlash(rel), Effects: append([]config.SharedExtraEffect{{Path: change.path, Before: before, After: change.after}}, change.effects...), MetadataAfter: change.metadata, RetainOnly: change.retainOnly}
		p.Parents = map[string]string{}
		for _, effect := range p.Effects {
			for parent := filepath.Dir(effect.Path); ; parent = filepath.Dir(parent) {
				fingerprint, err := placementFingerprint(parent)
				if err != nil {
					return r.latch(err)
				}
				p.Parents[parent] = fingerprint
				if filepath.Dir(parent) == parent {
					break
				}
			}
		}
		if p.MetadataAfter != nil {
			p.MetadataBefore, err = sharedNativeFacts(p)
			if err != nil {
				return r.latch(err)
			}
		}
		if change.acquireOnly {
			for name, expected := range p.MetadataAfter {
				if p.MetadataBefore[name] != expected {
					return r.latch(fmt.Errorf("native restore proof changed before adoption"))
				}
			}
		}
		if r.state.State.Pending != nil {
			return r.latch(fmt.Errorf("shared placement recovery is unresolved"))
		}
		r.state.State.Pending = p
		if err := r.Save(); err != nil {
			return err
		}
		if err := checkPlacementParents(p, false); err != nil {
			return r.latch(err)
		}
		for _, effect := range p.Effects {
			if got, err := placementFingerprint(effect.Path); err != nil || got != effect.Before {
				return r.latch(fmt.Errorf("shared destination changed before placement: %s", effect.Path))
			}
		}
		if current, err := sharedNativeFacts(p); err != nil {
			return r.latch(err)
		} else {
			for name, fingerprint := range p.MetadataBefore {
				if current[name] != fingerprint {
					return r.latch(fmt.Errorf("native %s metadata changed before placement", name))
				}
			}
		}
		nativeErr := mutate()
		err = r.recoverPending()
		if err != nil {
			return r.latch(errors.Join(nativeErr, err))
		}
		return nativeErr
	}
}

func placementFingerprint(path string) (string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	if utils.IsLinkMode(path, info.Mode()) {
		raw, err := os.Readlink(path)
		return "link:" + raw, err
	}
	if info.IsDir() {
		return "directory", nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("unsupported shared destination: %s", path)
	}
	hash, err := utils.FileHash(path)
	return "file:" + hash, err
}

func matchesPlacementFingerprint(path, got, want string) bool {
	if strings.HasPrefix(want, "link-to:") && strings.HasPrefix(got, "link:") {
		return utils.PathsEqual(evalOrClean(resolveReadlink(strings.TrimPrefix(got, "link:"), path)), strings.TrimPrefix(want, "link-to:"))
	}
	return got == want
}

func checkPlacementParents(p *config.SharedExtraIntent, recovery bool) error {
	if len(p.Effects) == 2 {
		link := resolveReadlink(strings.TrimPrefix(p.Effects[0].Before, "link:"), p.Effects[0].Path)
		destination, err := utils.ConfigWritePath(link, false)
		if err != nil {
			return err
		}
		if !utils.PathsEqual(destination, p.Effects[1].Path) {
			return fmt.Errorf("shared write referent changed")
		}
	}
	for path, before := range p.Parents {
		got, err := placementFingerprint(path)
		if err != nil {
			return err
		}
		if got != before && !(recovery && before == "absent" && got == "directory") {
			return fmt.Errorf("shared destination parent changed: %s", path)
		}
	}
	return nil
}

func (r *SharedExtrasRun) recoverPending() error {
	p := r.state.State.Pending
	if p == nil {
		return nil
	}
	if err := checkPlacementParents(p, true); err != nil {
		return err
	}
	before, after := true, true
	for _, effect := range p.Effects {
		got, err := placementFingerprint(effect.Path)
		if err != nil {
			return err
		}
		before = before && got == effect.Before && got != "directory"
		after = after && matchesPlacementFingerprint(effect.Path, got, effect.After)
	}
	if !before && !after {
		return fmt.Errorf("shared placement for %s at %s is ambiguous; kept for recovery", p.Placement.Name, p.Effects[0].Path)
	}
	if err := reconcileSharedNative(p, after); err != nil {
		return err
	}
	if after && !p.RetainOnly {
		if p.Placement.Receipt.File != "" {
			p.Placement.Receipt.Targets[0].Native = p.MetadataAfter
		}
		if p.Placement.Receipt.File == "" && p.Placement.Receipt.Targets[0].Mode != "symlink" {
			hash, err := sharedOutputIdentity(p.Effects[0].Path)
			if err != nil {
				return err
			}
			p.Placement.Receipt.Targets[0].Files = map[string]string{p.Relative: hash}
		}
		r.mergePlacement(p.Placement)
		if same, shared := r.currentTarget(p.Placement, p.Placement.Receipt.Targets[0], config.ExpandPath); same && shared {
			r.recovered[p.Effects[0].Path] = true
		}
	}
	r.state.State.Pending = nil
	if err := r.state.Save(); err != nil {
		r.state.State.Pending = p
		return err
	}
	return nil
}

// DetachExtraFileTarget uses restore's ownership tests but leaves edited
// outputs in place, and releases the old attach receipt exactly once.
func DetachExtraFileTarget(f ExtraFile) (changed, kept bool, err error) {
	if isContentBlockMode(f.Mode) {
		raw, readErr := os.ReadFile(f.Target)
		if os.IsNotExist(readErr) {
			return false, false, nil
		}
		if readErr != nil {
			return false, false, readErr
		}
		block, findErr := f.findContentBlock(strings.Split(string(raw), "\n"))
		if findErr != nil {
			return false, false, findErr
		}
		if block != nil && contentBlockHash(block.body) != block.hash {
			ForgetExtraTarget(f)
			return false, true, nil
		}
	} else if f.Mode != "import" {
		info, statErr := os.Lstat(f.Target)
		if statErr != nil && !os.IsNotExist(statErr) {
			return false, false, statErr
		}
		owned, drift := extraRestoreOwnership(f, info)
		if drift || info != nil && !owned {
			ForgetExtraTarget(f)
			return false, true, nil
		}
	}
	changed, err = RestoreExtraTarget(f)
	if err != nil {
		return false, false, fmt.Errorf("detach: %w", err)
	}
	return changed, false, nil
}
