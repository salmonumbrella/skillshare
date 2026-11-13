package sharedfile

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckReasons(t *testing.T) {
	token := strings.Repeat("aB7", 8)
	assignmentKey := "pass" + "word"
	cases := []struct {
		name, text string
		reason     Reason
	}{
		{"comment", "version: 1\n# " + "gh" + "p_" + strings.Repeat("a", 36), Privacy},
		{"plain assignment", "version: 1\npassword: " + token, Privacy},
		{"single assignment", "version: 1\npassword: '" + token + "'", Privacy},
		{"double assignment", "version: 1\npassword: \"" + token + "\"", Privacy},
		{"URL", "version: 1\n# https://alice:pw@example.com", Privacy},
		{"version", "version: 2", UnsupportedVersion},
		{"newer", "version: 1\nmcp: {}", NewerSection},
		{"machine", "version: 1\nsources: {}", MachineKey},
		{"unknown", "version: 1\ncolour: blue", UnknownKey},
		{"nested", "version: 1\nextras: [{name: rules, remote: somewhere}]", UnknownKey},
		{"combined key", "version: 1\nextras: [{name: rules, name source: ignored}]", UnknownKey},
		{"target key", "version: 1\nextras: [{name: rules, targets: [{path: rules, colour: blue}]}]", UnknownKey},
		{"list", "[one, two]", Parse},
		{"empty", "", Parse},
		{"two documents", "version: 1\n---\nversion: 1", Parse},
		{"alias", "version: 1\nextras: &x []\ncolour: *x", Parse},
		{"duplicate", "version: 1\nversion: 1", Parse},
		{"nested privacy wins", "version: 2\nextras: [{name: rules, " + assignmentKey + ": " + token + "}]", Privacy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Check("team.yaml", []byte(tc.text))
			if doc != nil || err == nil || err.Reason != tc.reason {
				t.Fatalf("doc=%v error=%v want=%s", doc, err, tc.reason)
			}
			rendered := fmt.Sprint(err, err.Findings)
			for i := 0; i+4 <= len(token); i++ {
				if strings.Contains(rendered, token[i:i+4]) {
					t.Fatal("diagnostic leaked value")
				}
			}
		})
	}
	for _, text := range []string{"version: 1\nextras: []\n", "\ufeffversion: 1\r\nextras: []\r\n"} {
		doc, err := Check("team.yaml", []byte(text))
		if err != nil || doc.Section("extras") == nil {
			t.Fatalf("valid document: %v", err)
		}
	}
}

func TestParseDiagnosticsDropParserText(t *testing.T) {
	value := "fixture" + strings.Repeat("Q", 25)
	_, err := Check("team.yaml", []byte("version: 1\nextras: [\n  "+value+": : broken"))
	if err == nil || err.Reason != Parse {
		t.Fatalf("error=%v", err)
	}
	text := fmt.Sprint(err, err.Findings)
	for i := 0; i+4 <= len(value); i++ {
		if strings.Contains(text, value[i:i+4]) {
			t.Fatal("parser value leaked")
		}
	}
}
