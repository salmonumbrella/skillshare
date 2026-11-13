package sync

import "skillshare/internal/config"

// ManagedExtraTargetFiles maps the filtered, flattened output names without
// running an extension. Its caller resolves the extension's output suffix.
func ManagedExtraTargetFiles(target config.ExtraTargetConfig, sourceDir, outputExt string) (map[string]bool, error) {
	files, _, err := DiscoverExtraTargetFiles(sourceDir, "", target)
	if err != nil {
		return nil, err
	}
	seen, managed := map[string]bool{}, map[string]bool{}
	for _, rel := range files {
		if name, ok := FlattenRel(rel, target.Flatten, seen); ok {
			managed[ApplyOutputExt(name, outputExt)] = true
		}
	}
	return managed, nil
}
