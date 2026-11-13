package audit

import "strings"

// SecretMatch deliberately contains no matched text or snippets.
type SecretMatch struct {
	Line   int
	RuleID string
}

// SecretMatches scans exact text, including comments, using only built-in
// hardcoded-secret rules. User overrides cannot weaken publication checks.
func SecretMatches(content []byte) ([]SecretMatch, error) {
	return secretMatches(content, loadBuiltinRules)
}

func secretMatches(content []byte, load func() ([]rule, error)) ([]SecretMatch, error) {
	rules, err := load()
	if err != nil {
		return nil, err
	}
	var matches []SecretMatch
	for i, line := range strings.Split(strings.TrimPrefix(string(content), "\ufeff"), "\n") {
		var lower string
		ready := false
		for _, r := range rules {
			if r.Pattern == "hardcoded-secret" && rulePrefilterAllows(r, line, &lower, &ready) && r.matchesLine(line) {
				matches = append(matches, SecretMatch{Line: i + 1, RuleID: r.ID})
			}
		}
	}
	return matches, nil
}
