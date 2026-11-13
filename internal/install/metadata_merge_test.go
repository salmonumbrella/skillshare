package install

import (
	"encoding/json"
	"testing"
)

func TestMergeMetadata_DeletesOnlyUntouchedEntries(t *testing.T) {
	base := []byte(`{"version":1,"entries":{"uninstalled-here":{"source":"a"},"deleted-remotely":{"source":"b"}}}`)
	ours := []byte(`{"version":1,"entries":{"deleted-remotely":{"source":"b2"}}}`)
	theirs := []byte(`{"version":1,"entries":{"uninstalled-here":{"source":"a"}}}`)

	out, err := MergeMetadata(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	var got MetadataStore
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Entries["uninstalled-here"]; ok {
		t.Error("expected a local uninstall of an entry the remote left alone to stick")
	}
	if e := got.Entries["deleted-remotely"]; e == nil || e.Source != "b2" {
		t.Errorf("expected the local edit to beat the remote delete, got %+v", e)
	}
}
