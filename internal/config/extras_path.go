package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveExtrasSourceDir resolves the source directory for an extra using
// three-level priority: per-extra source > extras_source > default.
// All paths are expected to be already expanded (no ~ tildes).
func ResolveExtrasSourceDir(extra ExtraConfig, extrasSource, skillsSource string) string {
	if extra.Source != "" {
		return extra.Source
	}
	if extrasSource != "" {
		return filepath.Join(extrasSource, extra.Name)
	}
	return filepath.Join(filepath.Dir(skillsSource), "extras", extra.Name)
}

// ExtrasSourceDirProject returns the source directory for a named extra in project mode.
// extrasParent is the resolved extras parent (e.g. from ProjectConfig.EffectiveExtrasSource).
func ExtrasSourceDirProject(extrasParent, name string) string {
	return filepath.Join(extrasParent, name)
}

// ResolveExtrasSourceDirProject resolves the source directory for an extra in
// project mode: its source (relative to the project root) when set, else
// <extrasParent>/<name>.
func ResolveExtrasSourceDirProject(extra ExtraConfig, extrasParent, root string) string {
	if extra.Source != "" {
		return filepath.Join(root, filepath.FromSlash(extra.Source))
	}
	return ExtrasSourceDirProject(extrasParent, extra.Name)
}

// ValidateProjectExtraSource checks a project extra's source: a path relative
// to the project root that stays inside it. Empty means the default folder.
func ValidateProjectExtraSource(source string) error {
	if source == "" {
		return nil
	}
	if filepath.IsAbs(source) || filepath.VolumeName(source) != "" || strings.HasPrefix(source, "~") || strings.ContainsAny(source[:1], `/\`) {
		return fmt.Errorf("extra source %q must be relative to the project root", source)
	}
	clean := filepath.Clean(filepath.FromSlash(source))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("extra source %q must stay inside the project root", source)
	}
	return nil
}

// ExtrasParentDir returns the extras parent directory (for migration/init).
func ExtrasParentDir(skillsSource string) string {
	return filepath.Join(filepath.Dir(skillsSource), "extras")
}

// ExtrasParentDirProject returns the extras parent directory in project mode.
// extrasParent is the resolved extras parent (e.g. from ProjectConfig.EffectiveExtrasSource).
func ExtrasParentDirProject(extrasParent string) string {
	return extrasParent
}

// ResolveExtrasSourceType returns which level resolved the source path.
// IMPORTANT: priority logic must mirror ResolveExtrasSourceDir above.
func ResolveExtrasSourceType(extra ExtraConfig, extrasSource string) string {
	if extra.Source != "" {
		return "per-extra"
	}
	if extrasSource != "" {
		return "extras_source"
	}
	return "default"
}
