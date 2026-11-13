package install

import (
	"path/filepath"
	"strings"
)

// SetTargetOverride records the targets for a skill inside a tracked repo,
// keyed by its source-relative path. An empty list is kept as an explicit
// "all targets" override so it still replaces the repo's frontmatter.
func (s *MetadataStore) SetTargetOverride(relPath string, targets []string) {
	if s.TargetOverrides == nil {
		s.TargetOverrides = make(map[string][]string)
	}
	if targets == nil {
		targets = []string{}
	}
	s.TargetOverrides[filepath.ToSlash(relPath)] = targets
}

// RemoveTargetOverrides drops the override for relPath and every skill below
// it, e.g. all skills of a tracked repo when the repo is uninstalled.
func (s *MetadataStore) RemoveTargetOverrides(relPath string) {
	relPath = filepath.ToSlash(relPath)
	for key := range s.TargetOverrides {
		if key == relPath || strings.HasPrefix(key, relPath+"/") {
			delete(s.TargetOverrides, key)
		}
	}
}

// LoadTargetOverrides reads the target overrides from dir's .metadata.json
// without migrating or rewriting the file. Returns nil when there are none.
func LoadTargetOverrides(dir string) map[string][]string {
	store, err := loadMetadataFile(dir)
	if err != nil {
		return nil
	}
	return store.TargetOverrides
}
