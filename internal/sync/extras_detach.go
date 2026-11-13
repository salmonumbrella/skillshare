package sync

import (
	"fmt"
	"os"
	"path/filepath"
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
	protected map[string]bool
}

func BeginSharedExtras(cfg *config.Config, dryRun bool) (*SharedExtrasRun, error) {
	if !cfg.SharedEnabled() {
		return nil, nil
	}
	if err := cfg.CheckSharedApply(); err != nil {
		return nil, err
	}
	path := cfg.SharedStatus().Path
	state, err := config.LoadSharedState(config.ConfigPath(), cfg.EffectiveGitRoot(), filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	state.RecordPointer(path)
	return &SharedExtrasRun{cfg: cfg, state: state, dryRun: dryRun, protected: map[string]bool{}}, nil
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
	return r.state.Save()
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
			same := mode == t.Mode && extra.File == old.Receipt.File && utils.PathsEqual(source, old.Receipt.Source)
			return same, r.cfg.ExtraOrigin(extra.Name) == sharedfile.Shared
		}
	}
	return false, false
}

func (r *SharedExtrasRun) Detach(resolve func(string) string) []DetachResult {
	if r == nil {
		return nil
	}
	var results []DetachResult
	var remaining []config.SharedPlacement
	for _, old := range r.state.State.Extras {
		keep := old
		keep.Receipt.Targets = nil
		for _, target := range old.Receipt.Targets {
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
	return results
}

func (r *SharedExtrasRun) detachTarget(old config.SharedPlacement, target config.SharedPlacedTarget) DetachResult {
	result := DetachResult{Name: old.Name, Target: target.Path}
	if old.Receipt.File != "" {
		f := NewExtraFile(old.Receipt.Source, old.Receipt.File, target.Path, target.As, target.Mode)
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
	case "copy":
		copies := loadCopyTracker(target.Path)
		for rel, expected := range target.Files {
			path := filepath.Join(target.Path, filepath.FromSlash(rel))
			// Receipts are private but still never authorize traversing outside a target.
			relative, err := filepath.Rel(target.Path, path)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				result.Errors = append(result.Errors, "invalid placement output path")
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
			if !info.Mode().IsRegular() {
				result.Kept = append(result.Kept, path)
				copies.forget(rel)
				continue
			}
			hash, err := utils.FileHash(path)
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
	default:
		copies := loadCopyTracker(target.Path)
		for rel := range target.Files {
			path := filepath.Join(target.Path, filepath.FromSlash(rel))
			if info, err := os.Lstat(path); err == nil && !utils.IsLinkMode(path, info.Mode()) && !copies.owns(rel) {
				result.Kept = append(result.Kept, path)
			} else if err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, err.Error())
			}
		}
		removedLinks, pruneErrors := PruneExtraTarget(target.Path, old.Receipt.Source, "merge")
		result.Removed = removedLinks
		result.Errors = append(result.Errors, pruneErrors...)
		// Report edited fallback copies before the tracker forgets them.
		for rel := range copies.m.Managed {
			if !copies.owns(rel) {
				if _, err := os.Lstat(filepath.Join(target.Path, rel)); err == nil {
					result.Kept = append(result.Kept, filepath.Join(target.Path, rel))
				}
			}
		}
		removed, err := copies.pruneOrphans(map[string]bool{}, false)
		result.Removed += len(removed)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
		if err := copies.save(); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}
	return result
}

func (r *SharedExtrasRun) Record(extra config.ExtraConfig, sourceDir string, run ExtraRun, outputExt func(config.ExtraTargetConfig) string) {
	if r == nil || r.dryRun || run.SourceMissing || r.cfg.ExtraOrigin(extra.Name) != sharedfile.Shared {
		return
	}
	declaration, ok := r.cfg.SharedExtraDeclaration(extra.Name)
	if !ok {
		return
	}
	index := -1
	for i, old := range r.state.State.Extras {
		if old.Name == extra.Name && old.Receipt.Source == sourceDir && old.Receipt.File == extra.File {
			index = i
			break
		}
	}
	next := config.SharedPlacement{Name: extra.Name, Declaration: declaration, Receipt: config.SharedReceipt{Source: sourceDir, File: extra.File}}
	if index >= 0 {
		next = r.state.State.Extras[index]
		next.Declaration = declaration
	}
	for _, tr := range run.Targets {
		if tr.Err != nil || tr.ExtensionErr != nil || tr.ModeErr != nil || tr.SkippedBy != "" || tr.Result == nil || len(tr.Result.Errors) > 0 || tr.Result.Skipped > 0 {
			continue
		}
		target := config.SharedPlacedTarget{Path: tr.Path, Mode: tr.Mode, As: tr.Target.As}
		if protectedOutput(placedOutput(extra.File, target), r.protected) {
			continue
		}
		previous := -1
		for i, old := range next.Receipt.Targets {
			if old.Path == target.Path && old.As == target.As {
				previous = i
				break
			}
		}
		if extra.File == "" && (target.Mode == "copy" || target.Mode == "merge") {
			target.Files = map[string]string{}
			if previous >= 0 {
				for rel, hash := range next.Receipt.Targets[previous].Files {
					target.Files[rel] = hash
				}
			}
			valid := true
			for _, rel := range tr.Result.written {
				hash, err := utils.FileHash(filepath.Join(tr.Path, rel))
				if err != nil {
					valid = false
					break
				}
				target.Files[filepath.ToSlash(rel)] = hash
			}
			if !valid || len(target.Files) == 0 {
				continue
			}
		} else if extra.File != "" && !extraAttached(placedOutput(extra.File, target)) && !NewExtraFile(sourceDir, extra.File, tr.Path, tr.Target.As, tr.Mode).isOurLink() {
			// An identical file that SyncExtraFile did not attach remains the user's.
			continue
		}
		if previous < 0 {
			next.Receipt.Targets = append(next.Receipt.Targets, target)
		} else {
			next.Receipt.Targets[previous] = target
		}
	}
	if len(next.Receipt.Targets) == 0 {
		return
	}
	if index < 0 {
		r.state.State.Extras = append(r.state.State.Extras, next)
	} else {
		r.state.State.Extras[index] = next
	}
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
