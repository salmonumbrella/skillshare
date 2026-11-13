package sharedfile

import "path"

type Origin string

const (
	Shared          Origin = "shared"
	Local           Origin = "local"
	OverridesShared Origin = "overrides shared"
	Skipped         Origin = "skipped"
)

func ValidSkipPattern(p string) error { _, err := path.Match(p, ""); return err }
func SkipMatches(patterns []string, name string) bool {
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if match, _ := path.Match(p, name); match {
			return true
		}
	}
	return false
}

// Merge preserves shared declaration order followed by the whole local entries.
func Merge[T any](shared, local []T, name func(T) string, skip []string) (effective []T, origins map[string]Origin) {
	origins = map[string]Origin{}
	localNames := map[string]bool{}
	for _, entry := range local {
		localNames[name(entry)] = true
	}
	for _, entry := range shared {
		key := name(entry)
		switch {
		case localNames[key]:
			origins[key] = OverridesShared
		case SkipMatches(skip, key):
			origins[key] = Skipped
		default:
			origins[key] = Shared
			effective = append(effective, entry)
		}
	}
	for _, entry := range local {
		key := name(entry)
		if origins[key] != OverridesShared {
			origins[key] = Local
		}
		effective = append(effective, entry)
	}
	return effective, origins
}
