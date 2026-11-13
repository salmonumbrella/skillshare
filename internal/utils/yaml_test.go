package utils

import "testing"

func TestMarshalYAML_TwoSpaceIndent(t *testing.T) {
	got, err := MarshalYAML(map[string]any{"a": map[string]any{"b": []string{"c"}}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "a:\n  b:\n    - c\n"; string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
