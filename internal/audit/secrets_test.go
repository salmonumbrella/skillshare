package audit

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSecretMatchesBuiltinOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(dir, "config.yaml"))
	if err := os.WriteFile(filepath.Join(dir, "audit-rules.yaml"), []byte("rules:\n  - id: hardcoded-secret-9\n    enabled: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("aB7", 8)
	for _, tc := range []struct {
		text string
		want int
	}{
		{"password: " + token, 0},
		{"\ufeff# fixture\r\npassword: \"" + token + "\"\r\n", 1},
	} {
		got, err := SecretMatches([]byte(tc.text))
		if err != nil || len(got) != tc.want {
			t.Fatalf("matches=%v err=%v", got, err)
		}
		if len(got) > 0 && (got[0].Line != 2 || got[0].RuleID != "hardcoded-secret-9") {
			t.Fatalf("location: %v", got)
		}
	}
	typ := reflect.TypeOf(SecretMatch{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() == reflect.String && f.Name != "RuleID" {
			t.Fatalf("unsafe field: %s", f.Name)
		}
	}
}

func TestSecretMatchesRuleFailure(t *testing.T) {
	want := errors.New("fixture rule failure")
	got, err := secretMatches(nil, func() ([]rule, error) { return nil, want })
	if !errors.Is(err, want) || got != nil {
		t.Fatalf("matches=%v err=%v", got, err)
	}
}
