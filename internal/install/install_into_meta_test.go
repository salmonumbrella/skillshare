package install

import (
	"path/filepath"
	"testing"
)

// A discovery install into a group must record its metadata at the skills
// root, as Install does. With the metadata in the group folder instead, the
// skill never reaches config.yaml and teammates never install it.
func TestInstallFromDiscovery_IntoWritesMetadataAtSkillsRoot(t *testing.T) {
	url, _, _ := setupPinRemote(t)
	source, err := ParseSource(url)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)
	if len(discovery.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(discovery.Skills))
	}
	root := t.TempDir()
	dest := filepath.Join(root, "frontend", "react", "demo")

	// SourceDir is left empty on purpose: several callers do not set it.
	if _, err := InstallFromDiscovery(discovery, discovery.Skills[0], dest, InstallOptions{Into: "frontend/react", SkipAudit: true}); err != nil {
		t.Fatal(err)
	}

	if LoadMetadataOrNew(root).Get("frontend/react/demo") == nil {
		t.Fatalf("no metadata entry at the skills root; group folder has %v", LoadMetadataOrNew(filepath.Dir(dest)).List())
	}
}
