package install

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
)

// webRef is the "{ref}/{path}" tail of a GitHub, GitLab or Bitbucket web URL,
// such as "v1.2.0/skills/foo" from github.com/o/r/tree/v1.2.0/skills/foo.
type webRef struct {
	tail string
	blob bool
}

// split returns the path left after ref, or ok=false when ref is not a
// leading run of whole segments of the tail.
func (w webRef) split(ref string) (subdir string, explicit, ok bool) {
	var rest string
	switch {
	case w.tail == ref:
	case strings.HasPrefix(w.tail, ref+"/"):
		rest = w.tail[len(ref)+1:]
	default:
		return "", false, false
	}
	subdir, explicit = trimSkillFileSuffix(rest, w.blob)
	return subdir, explicit, true
}

// applyWebRef takes the tail's first segment as the ref and returns the subdir
// after it. The ref (branch, tag or commit SHA) becomes the clone ref, so a
// pasted or hub-listed URL installs the version it names. A branch name that
// contains "/" is corrected later by resolveWebRef. HEAD, as in GitHub's
// tree/HEAD links, means the remote's default branch.
func (s *Source) applyWebRef(w webRef) string {
	s.webRef = w
	if w.tail == "" {
		return ""
	}
	ref, _, _ := strings.Cut(w.tail, "/")
	subdir, explicit, _ := w.split(ref)
	s.ExplicitSkill = explicit
	if ref == "HEAD" {
		s.webRef = webRef{}
		return subdir
	}
	s.Branch = ref
	return subdir
}

// RemoteRefs caches each remote's branch and tag names, so resolving many web
// URLs from one repo lists its refs once. The zero value is ready to use.
type RemoteRefs struct {
	byURL map[string]*RefList
}

// RefList is a remote's default branch, branches and tags. Branches are
// sorted by name; tags newest version first.
type RefList struct {
	DefaultBranch string
	Branches      []string
	Tags          []string
}

func (l *RefList) has(ref string) bool {
	return slices.Contains(l.Branches, ref) || slices.Contains(l.Tags, ref)
}

// List returns the source remote's refs, listing each remote once.
func (r *RemoteRefs) List(s *Source) (*RefList, error) {
	if refs, ok := r.byURL[s.CloneURL]; ok {
		return refs, nil
	}
	refs, err := listRemoteRefs(s)
	if err != nil {
		return nil, err
	}
	if r.byURL == nil {
		r.byURL = make(map[string]*RefList)
	}
	r.byURL[s.CloneURL] = refs
	return refs, nil
}

func resolveWebRef(s *Source) error {
	return (&RemoteRefs{}).Resolve(s)
}

// Resolve settles where the ref ends in a web URL's "{ref}/{path}" when the
// ref may contain "/", as in tree/feature/x/skills/foo. A Branch that already
// covers more of the tail (from --branch or a saved config) only moves the
// subdir. Otherwise the remote's branches and tags decide. A URL ref that
// matches none of them fails instead of installing something else, unless an
// unrelated --branch was given, which then only borrows the URL's subdir.
// Once settled, the source no longer reports HasAmbiguousWebRef.
func (r *RemoteRefs) Resolve(s *Source) error {
	w := s.webRef
	if !w.ambiguous() {
		return nil
	}
	first, _, _ := strings.Cut(w.tail, "/")
	if s.Branch != "" && s.Branch != first && w.covers(s.Branch) {
		return s.settleWebRef(s.Branch)
	}
	if IsCommitSHA(first) {
		return s.settleWebRef(first)
	}

	refs, err := r.List(s)
	if err != nil {
		return nil // let the clone report the real problem
	}
	if refs.has(first) {
		return s.settleWebRef(first)
	}
	override := s.Branch != "" && s.Branch != first
	segments := strings.Split(w.tail, "/")
	for i := len(segments); i > 1; i-- {
		if ref := strings.Join(segments[:i], "/"); refs.has(ref) {
			if !override {
				s.Branch = ref
			}
			return s.settleWebRef(ref)
		}
	}
	if override {
		return nil
	}
	return fmt.Errorf("ref %q from the URL was not found on the remote; pass --branch to choose one (needed when a branch name contains \"/\")", first)
}

// Settle resolves the source's URL ref like Resolve, but reads a ref the
// remote does not have, or a remote that cannot be listed, as the URL's first
// segment, so the source can be re-pinned with AtRef.
func (r *RemoteRefs) Settle(s *Source) error {
	if err := r.Resolve(s); err == nil && !s.HasAmbiguousWebRef() {
		return nil
	}
	first, _, _ := strings.Cut(s.webRef.tail, "/")
	return s.settleWebRef(first)
}

// HasAmbiguousWebRef reports whether the source's URL ref might contain "/",
// so its real ref and subdir are only known after Resolve.
func (s *Source) HasAmbiguousWebRef() bool {
	return s.webRef.ambiguous()
}

func (w webRef) ambiguous() bool {
	return strings.Contains(w.tail, "/")
}

func (w webRef) covers(ref string) bool {
	return w.tail == ref || strings.HasPrefix(w.tail, ref+"/")
}

// settleWebRef moves Subdir, and a Name derived from it, to the path after ref
// and marks the web ref resolved. A repo-root copy (Subdir cleared for a
// whole-repo clone) keeps its root.
func (s *Source) settleWebRef(ref string) error {
	if s.Subdir != "" {
		subdir, explicit, _ := s.webRef.split(ref)
		if subdir != "" {
			if err := validateRepoSubdir(subdir); err != nil {
				return err
			}
		}
		if s.Name == path.Base(s.Subdir) {
			if subdir != "" {
				s.Name = path.Base(subdir)
			} else {
				s.Name = strings.TrimSuffix(path.Base(s.CloneURL), ".git")
			}
		}
		s.Subdir = subdir
		s.ExplicitSkill = explicit
	}
	s.webRef = webRef{}
	return nil
}

// listRemoteRefs returns the remote's default branch, branches and tags.
func listRemoteRefs(s *Source) (*RefList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, "ls-remote", "--symref", s.CloneURL, "HEAD", "refs/heads/*", "refs/tags/*")
	cmd.Env = append(cmd.Env, s.authEnv()...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-remote: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git ls-remote: %w", err)
	}
	refs := &RefList{Branches: []string{}, Tags: []string{}}
	for _, line := range strings.Split(string(out), "\n") {
		value, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		if target, ok := strings.CutPrefix(value, "ref: refs/heads/"); ok && name == "HEAD" {
			refs.DefaultBranch = target
		} else if b, ok := strings.CutPrefix(name, "refs/heads/"); ok {
			refs.Branches = append(refs.Branches, b)
		} else if t, ok := strings.CutPrefix(name, "refs/tags/"); ok && !strings.HasSuffix(t, "^{}") {
			refs.Tags = append(refs.Tags, t)
		}
	}
	slices.Sort(refs.Branches)
	slices.SortFunc(refs.Tags, compareTagsNewestFirst)
	return refs, nil
}

// compareTagsNewestFirst orders version tags (v1.2.0, 1.2) by descending
// version, before any other tags, which follow in reverse name order.
func compareTagsNewestFirst(a, b string) int {
	va, okA := parseTagVersion(a)
	vb, okB := parseTagVersion(b)
	switch {
	case okA && okB:
		if c := slices.Compare(vb.numbers, va.numbers); c != 0 {
			return c
		}
		// A release sorts before its pre-releases.
		if (va.pre == "") != (vb.pre == "") {
			if va.pre == "" {
				return -1
			}
			return 1
		}
		return strings.Compare(b, a)
	case okA:
		return -1
	case okB:
		return 1
	}
	return strings.Compare(b, a)
}

type tagVersion struct {
	numbers []int
	pre     string
}

func parseTagVersion(tag string) (tagVersion, bool) {
	core, pre, _ := strings.Cut(strings.TrimPrefix(tag, "v"), "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return tagVersion{}, false
	}
	numbers := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return tagVersion{}, false
		}
		numbers[i] = n
	}
	return tagVersion{numbers: numbers, pre: pre}, true
}

// ApplyRecordedBranch sets the branch a skill was installed with, for
// reinstalling it from its recorded source URL. An empty branch means the
// remote default even when the URL names a ref: installs from before URL refs
// were honoured recorded none, and updating them must not switch branch. A
// recorded branch that covers the URL's ref settles the subdir without asking
// the remote.
func (s *Source) ApplyRecordedBranch(branch string) {
	s.Branch = branch
	switch {
	case branch == "":
		s.webRef = webRef{}
	case s.webRef.covers(branch):
		// On a validation error the web ref stays, so install reports it.
		_ = s.settleWebRef(branch)
	}
}

// ErrRefNotPinnable reports a source whose URL cannot name a ref: only
// GitHub, GitLab and Bitbucket web URLs can.
var ErrRefNotPinnable = errors.New("only GitHub, GitLab and Bitbucket sources can pin a ref")

// webRepo returns the repo's web URL and the path markers that come before a
// ref in tree and file URLs, or ok=false for hosts whose URLs cannot name a ref.
func (s *Source) webRepo() (repo, tree, blob string, ok bool) {
	base := strings.TrimSuffix(s.CloneURL, ".git")
	switch {
	case s.Type == SourceTypeGitHub:
		return strings.TrimPrefix(base, "https://"), "tree", "blob", true
	case s.Type == SourceTypeGitHTTPS && s.webHost == "gitlab":
		return base, "-/tree", "-/blob", true
	case s.Type == SourceTypeGitHTTPS && s.webHost == "bitbucket":
		return base, "src", "src", true
	}
	return "", "", "", false
}

// PinnedRef returns the ref the source's URL names ("" for the remote
// default branch), and ok=false when the source cannot name one. A ref that
// may contain "/" is only its first segment until RemoteRefs.Resolve.
func (s *Source) PinnedRef() (ref string, ok bool) {
	if _, _, _, ok := s.webRepo(); !ok {
		return "", false
	}
	return s.Branch, true
}

// AtRef returns the source as a web URL that pins ref, keeping its subdir and
// SKILL.md target. An empty ref means the remote default branch.
func (s *Source) AtRef(ref string) (string, error) {
	repo, tree, blob, ok := s.webRepo()
	if !ok {
		return "", ErrRefNotPinnable
	}
	if s.HasAmbiguousWebRef() {
		return "", fmt.Errorf("the ref in %q could not be resolved", s.Raw)
	}
	if ref != "" && (strings.ContainsAny(ref, " \t\r\n?#\\") || strings.Contains(ref, "..") ||
		strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") || strings.HasPrefix(ref, "-")) {
		return "", fmt.Errorf("invalid ref %q", ref)
	}
	marker, tail := tree, s.Subdir
	if s.ExplicitSkill {
		marker, tail = blob, path.Join(s.Subdir, "SKILL.md")
	}
	if ref == "" {
		// GitHub URLs need no ref. Elsewhere a repo root keeps ".git" and a path
		// gets HEAD for the default branch, so a path joined on later is not
		// read as part of the repo.
		if !s.ExplicitSkill && s.Type == SourceTypeGitHub {
			return joinURLPath(repo, s.Subdir), nil
		}
		if !s.ExplicitSkill && s.Subdir == "" {
			return repo + ".git", nil
		}
		ref = "HEAD"
	}
	return joinURLPath(repo, marker, ref, tail), nil
}

func joinURLPath(parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(p string) bool { return p == "" }), "/")
}

// SourceAtRef rewrites a GitHub, GitLab or Bitbucket source as a web URL
// pinned at ref (the remote default branch when ref is empty). A URL ref that
// may contain "/" is settled against the remote first.
func SourceAtRef(raw, ref string, opts ParseOptions) (string, error) {
	s, err := ParseSourceWithOptions(raw, opts)
	if err != nil {
		return "", err
	}
	if _, _, _, ok := s.webRepo(); !ok {
		return "", ErrRefNotPinnable
	}
	if err := (&RemoteRefs{}).Settle(s); err != nil {
		return "", err
	}
	return s.AtRef(ref)
}

// SourceRef returns the ref a source's URL names, or "" for the default
// branch, other hosts and unparseable sources. A ref that may contain "/" is
// settled against the remote, listed once per remote through refs; with nil
// refs it stays the URL's first segment and nothing is asked of the remote.
func SourceRef(raw string, opts ParseOptions, refs *RemoteRefs) string {
	s, err := ParseSourceWithOptions(raw, opts)
	if err != nil {
		return ""
	}
	if _, ok := s.PinnedRef(); !ok {
		return ""
	}
	if refs != nil {
		// Settle only fails on an invalid subdir, which install reports; the
		// ref it settled on still stands.
		_ = refs.Settle(s)
	}
	ref, _ := s.PinnedRef()
	return ref
}
