package config

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"
	"skillshare/internal/sharedfile"
	"skillshare/internal/utils"
)

// Declarations remain portable; receipts describe only what this machine placed.
type SharedPlacedTarget struct {
	Path   string            `yaml:"path"`
	Mode   string            `yaml:"mode,omitempty"`
	As     string            `yaml:"as,omitempty"`
	Native map[string]string `yaml:"native,omitempty"`
	Files  map[string]string `yaml:"files,omitempty"` // written relative outputs: content hashes or link identities
}
type SharedReceipt struct {
	Source  string               `yaml:"source"`
	File    string               `yaml:"file,omitempty"`
	Targets []SharedPlacedTarget `yaml:"targets,omitempty"`
}
type SharedPlacement struct {
	Name           string        `yaml:"name"`
	Declaration    yaml.Node     `yaml:"declaration"`
	DeclarationDir string        `yaml:"declaration_dir,omitempty"`
	Receipt        SharedReceipt `yaml:"receipt"`
}

// SharedExtraIntent is one native output awaiting its durable receipt. It holds
// identities and native bookkeeping facts, never output or source content.
type SharedExtraIntent struct {
	Placement      SharedPlacement     `yaml:"placement"`
	Relative       string              `yaml:"relative,omitempty"`
	Effects        []SharedExtraEffect `yaml:"effects"`
	Parents        map[string]string   `yaml:"parents,omitempty"`
	MetadataBefore map[string]string   `yaml:"metadata_before,omitempty"`
	MetadataAfter  map[string]string   `yaml:"metadata_after,omitempty"`
	Fallback       bool                `yaml:"fallback,omitempty"`
	RetainOnly     bool                `yaml:"retain_only,omitempty"`
}
type SharedExtraEffect struct {
	Path   string `yaml:"path"`
	Before string `yaml:"before"`
	After  string `yaml:"after"`
}
type SharedState struct {
	Version  int                `yaml:"version"`
	Pointer  string             `yaml:"pointer,omitempty"`
	Pointers []string           `yaml:"pointers,omitempty"`
	Extras   []SharedPlacement  `yaml:"extras,omitempty"`
	Pending  *SharedExtraIntent `yaml:"pending,omitempty"`
}
type SharedStateFile struct {
	State      SharedState
	path       string
	configPath string
	roots      []string
	raw        []byte
	guard      *sharedWriteGuard
	writeFault func() error
}

type sharedWriteGuard struct {
	file           *SharedStateFile
	leased, closed bool
}

func (g *sharedWriteGuard) acquire(ctx context.Context) (func(), error) {
	if g.closed {
		return nil, fmt.Errorf("shared state lease is closed")
	}
	if g.leased {
		return func() {}, nil
	}
	f := g.file
	if err := f.safePath(); err != nil {
		return nil, err
	}
	if _, err := readSharedState(f.path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0700); err != nil {
		return nil, err
	}
	lockPath := strings.TrimSuffix(f.path, ".yaml") + ".lock"
	if info, err := os.Lstat(lockPath); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("shared state lock must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lock := flock.New(lockPath, flock.SetPermissions(0600))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		_ = lock.Close()
		if err == nil {
			err = ctx.Err()
		}
		return nil, fmt.Errorf("shared state lock: %w", err)
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = lock.Close() }) }
	if err := f.safePath(); err != nil {
		release()
		return nil, err
	}
	if _, err := readSharedState(f.path); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// BeginSharedStateWrite leases one resolved config identity, then reloads it.
// Read-only loads and candidate preparation do not acquire this lease.
func BeginSharedStateWrite(ctx context.Context, configPath, gitRoot, sharedDir string) (*SharedStateFile, func(), error) {
	f, err := LoadSharedState(configPath, gitRoot, sharedDir)
	if err != nil {
		return nil, nil, err
	}
	release, err := f.guard.acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	next, err := LoadSharedState(configPath, gitRoot, sharedDir)
	if err != nil {
		release()
		return nil, nil, err
	}
	if !utils.PathsEqual(next.path, f.path) {
		release()
		return nil, nil, fmt.Errorf("shared state identity changed; retry")
	}
	next.guard.leased = true
	var once sync.Once
	return next, func() { once.Do(func() { next.guard.closed = true; release() }) }, nil
}

// SetWriteFaultForTest injects a fault into this file's saves only.
func (f *SharedStateFile) SetWriteFaultForTest(fault func() error) { f.writeFault = fault }

func SharedStatePath(configPath string) (string, error) {
	return sharedStatePath(configPath, runtime.GOOS)
}
func sharedStatePath(configPath, goos string) (string, error) {
	id, err := projectConfigIdentity(configPath, goos)
	if err != nil {
		return "", err
	}
	return filepath.Join(projectStateDir(goos), "shared", id+".yaml"), nil
}

// LoadSharedState never writes, including when the state file does not exist.
func LoadSharedState(configPath, gitRoot, sharedDir string) (*SharedStateFile, error) {
	path, err := SharedStatePath(configPath)
	if err != nil {
		return nil, err
	}
	f := &SharedStateFile{path: path, configPath: configPath, roots: []string{gitRoot, sharedDir}}
	f.guard = &sharedWriteGuard{file: f}
	if err := f.safePath(); err != nil {
		return nil, err
	}
	raw, err := readSharedState(path)
	if err != nil {
		return nil, err
	}
	state, err := decodeSharedState(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	f.State, f.raw = state, raw
	return f, nil
}

func (f *SharedStateFile) safePath() error {
	current, err := SharedStatePath(f.configPath)
	if err != nil {
		return err
	}
	if !utils.PathsEqual(current, f.path) {
		return fmt.Errorf("machine-local shared state location changed; reload it")
	}
	stateDir, err := utils.ConfigWritePath(filepath.Dir(f.path), false)
	if err != nil {
		return err
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return err
	}
	for _, root := range f.roots {
		if root == "" {
			continue
		}
		resolved, err := utils.ConfigWritePath(root, false)
		if err != nil {
			return err
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return err
		}
		if utils.PathsEqual(stateDir, resolved) || utils.PathHasPrefix(stateDir, strings.TrimRight(resolved, `/\`)+string(filepath.Separator)) {
			return fmt.Errorf("machine-local shared state directory %s is inside published directory %s", filepath.Dir(f.path), root)
		}
	}
	return nil
}

func readSharedState(path string) ([]byte, error) {
	for _, p := range []string{filepath.Dir(path), path} {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("machine-local shared state must not be a symlink: %s", p)
		}
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return raw, err
}

func decodeSharedState(raw []byte) (SharedState, error) {
	state := SharedState{Version: 1}
	if raw == nil {
		return state, nil
	}
	state.Version = 0
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&state); err != nil {
		return state, fmt.Errorf("invalid machine-local shared state")
	}
	if dec.Decode(new(yaml.Node)) != io.EOF {
		return state, fmt.Errorf("machine-local shared state must contain one YAML document")
	}
	if state.Version != 1 {
		return state, fmt.Errorf("unsupported machine-local shared state version")
	}
	placements := slices.Clone(state.Extras)
	if p := state.Pending; p != nil {
		placements = append(placements, p.Placement)
		if len(p.Placement.Receipt.Targets) != 1 || p.Placement.DeclarationDir == "" || len(p.Effects) < 1 || len(p.Effects) > 2 {
			return state, fmt.Errorf("invalid shared placement intent")
		}
		target := p.Placement.Receipt.Targets[0]
		if ValidateExtraConfig(ExtraConfig{Name: p.Placement.Name, File: p.Placement.Receipt.File, Targets: []ExtraTargetConfig{{Path: target.Path, Mode: target.Mode, As: target.As}}}) != nil {
			return state, fmt.Errorf("invalid shared intent target")
		}
		if len(p.Effects) == 2 && !strings.HasPrefix(p.Effects[0].Before, "link:") {
			return state, fmt.Errorf("invalid shared intent referent")
		}
		rel := filepath.FromSlash(p.Relative)
		if !filepath.IsAbs(target.Path) || p.Relative != "" && (filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return state, fmt.Errorf("invalid shared intent output")
		}
		output := target.Path
		if p.Placement.Receipt.File != "" {
			name := target.As
			if name == "" {
				name = p.Placement.Receipt.File
			}
			output = filepath.Join(output, name)
		} else {
			output = filepath.Join(output, rel)
		}
		if !utils.PathsEqual(p.Effects[0].Path, output) || !filepath.IsAbs(p.Placement.Receipt.Source) {
			return state, fmt.Errorf("invalid shared intent destination")
		}
		for _, effect := range p.Effects {
			if !filepath.IsAbs(effect.Path) || !sharedFingerprintValid(effect.Before) || !sharedFingerprintValid(effect.After) || effect.After == "absent" || effect.After == "directory" {
				return state, fmt.Errorf("invalid shared intent fingerprint")
			}
		}
		for path, fingerprint := range p.Parents {
			if !filepath.IsAbs(path) || !(fingerprint == "absent" || fingerprint == "directory" || strings.HasPrefix(fingerprint, "link:")) || !sharedFingerprintValid(fingerprint) {
				return state, fmt.Errorf("invalid shared intent parent")
			}
		}
		if len(p.MetadataBefore) != len(p.MetadataAfter) || !sharedMetadataValid(p.MetadataBefore) || !sharedMetadataValid(p.MetadataAfter) {
			return state, fmt.Errorf("invalid shared intent metadata")
		}
		for name := range p.MetadataBefore {
			if _, ok := p.MetadataAfter[name]; !ok {
				return state, fmt.Errorf("invalid shared intent metadata")
			}
		}

	}
	for _, placement := range placements {
		if ValidateExtraName(placement.Name) != nil {
			return state, fmt.Errorf("invalid shared placement name")
		}
		if placement.DeclarationDir != "" && !filepath.IsAbs(placement.DeclarationDir) {
			return state, fmt.Errorf("invalid shared declaration directory")
		}
		for _, target := range placement.Receipt.Targets {
			if !sharedMetadataValid(target.Native) {
				return state, fmt.Errorf("invalid shared native receipt")
			}
		}
		// Reuse the portable validator so raw declaration nodes cannot hide unknown
		// fields, aliases or invalid extras shapes inside a machine-local snapshot.
		root := yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "version"}, {Kind: yaml.ScalarNode, Tag: "!!int", Value: "1"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "extras"}, {Kind: yaml.SequenceNode, Content: []*yaml.Node{&placement.Declaration}},
		}}
		data, err := yaml.Marshal(&root)
		if err != nil {
			return state, fmt.Errorf("invalid shared placement declaration")
		}
		doc, unavailable := sharedfile.Check("state declaration", data)
		if unavailable != nil {
			return state, fmt.Errorf("invalid shared placement declaration")
		}
		entries, unavailable := decodeSharedExtras(doc)
		if unavailable != nil || len(entries) != 1 || entries[0].Name != placement.Name {
			return state, fmt.Errorf("invalid shared placement declaration")
		}
	}
	return state, nil
}

func sharedMetadataValid(metadata map[string]string) bool {
	for name, fingerprint := range metadata {
		if !slices.Contains([]string{"created", "restore", "restore-link", "restore-junction", "written", "import-base", "fallback"}, name) || !(fingerprint == "absent" || strings.HasPrefix(fingerprint, "file:")) || !sharedFingerprintValid(fingerprint) {
			return false
		}
	}
	return true
}

func sharedFingerprintValid(value string) bool {
	if value == "absent" || value == "directory" {
		return true
	}
	if strings.HasPrefix(value, "link-to:") {
		return filepath.IsAbs(strings.TrimPrefix(value, "link-to:"))
	}
	if strings.HasPrefix(value, "link:") {
		return len(value) > 5 && !strings.ContainsRune(value, 0)
	}
	if strings.HasPrefix(value, "file:") && len(value) == 69 {
		_, err := hex.DecodeString(value[5:])
		return err == nil
	}
	return false
}

func (f *SharedStateFile) RecordPointer(resolved string) {
	if resolved != "" && !slices.Contains(f.State.Pointers, resolved) {
		f.State.Pointers = append(f.State.Pointers, resolved)
	}
	f.State.Pointer = resolved
}

// stateCandidate reuses the existing private atomic writer and snapshot check.
func (f *SharedStateFile) stateCandidate() (*SaveCandidate, error) {
	state := f.State
	if state.Extras != nil {
		state.Extras = append([]SharedPlacement(nil), state.Extras...)
	}
	for i := range state.Extras {
		state.Extras[i].Declaration = withoutSharedComments(state.Extras[i].Declaration)
	}
	if state.Pending != nil {
		pending := *state.Pending
		pending.Placement.Declaration = withoutSharedComments(pending.Placement.Declaration)
		state.Pending = &pending
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("cannot encode machine-local shared state")
	}
	if _, err := decodeSharedState(data); err != nil {
		return nil, err
	}
	c := &SaveCandidate{path: f.path, Bytes: data, private: true, guard: f.guard}
	observed := bytes.Clone(f.raw)
	c.check = func() error {
		if err := f.safePath(); err != nil {
			return err
		}
		raw, err := readSharedState(f.path)
		if err != nil {
			return err
		}
		if (raw == nil) != (observed == nil) || !bytes.Equal(raw, observed) {
			return fmt.Errorf("machine-local shared state changed; retry the command")
		}
		return nil
	}
	return c, nil
}
func withoutSharedComments(n yaml.Node) yaml.Node {
	n.HeadComment, n.LineComment, n.FootComment = "", "", ""
	if n.Content != nil {
		children := make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			copy := withoutSharedComments(*child)
			children[i] = &copy
		}
		n.Content = children
	}
	return n
}
func (f *SharedStateFile) Save() error {
	if f.writeFault != nil {
		if err := f.writeFault(); err != nil {
			return err
		}
	}
	candidate, err := f.stateCandidate()
	if err != nil {
		return err
	}
	if err := candidate.Write(); err != nil {
		return err
	}
	f.raw = append([]byte(nil), candidate.Bytes...)
	return nil
}

// PrepareRawConfigSave keeps the global raw editor's bytes verbatim. Project
// configs retain their separate loader and writer; sharing is global only.
func PrepareRawConfigSave(path string, raw []byte) (*SaveCandidate, error) {
	c := &SaveCandidate{path: path, Bytes: append([]byte(nil), raw...)}
	c.Expanded, c.Err = DecodeReadOnly(path, c.Bytes)
	if c.Err != nil {
		return c, c.Err
	}
	previous, err := readConfigSnapshot(path)
	if err == nil {
		err = c.prepareSharedHistory(previous)
	}
	c.Err = err
	return c, err
}

func readConfigSnapshot(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return raw, err
}

// Only shared.path is decoded from the old file. This reads no shared
// document and ignores future local sections; malformed old YAML is handled
// as an unobserved pointer by prepareSharedHistory.
func pointerFromConfig(raw []byte) (*SharedPointer, error) {
	if raw == nil {
		return nil, nil
	}
	var doc struct {
		Shared struct {
			Path yaml.Node `yaml:"path"`
		} `yaml:"shared"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot read previous shared.path")
	}
	p := doc.Shared.Path
	if p.Kind == 0 {
		return nil, nil
	}
	if p.Kind != yaml.ScalarNode || p.Tag != "!!str" || strings.TrimSpace(p.Value) == "" || strings.ContainsRune(p.Value, 0) {
		return nil, fmt.Errorf("cannot read previous shared.path")
	}
	return &SharedPointer{Path: p.Value}, nil
}

// prepareSharedHistory attaches the sole pre-write owner of pointer history.
func (c *SaveCandidate) prepareSharedHistory(previous []byte) error {
	// A malformed old document has no interpretable pointer. Raw repair must
	// remain available, independent of comments or quoting. Previously recorded
	// history survives; an unobserved invalid hand edit cannot be reconstructed.
	old, err := pointerFromConfig(previous)
	if err != nil {
		old = nil
	}
	var next struct {
		Shared *SharedPointer `yaml:"shared"`
	}
	if err := yaml.Unmarshal(c.Bytes, &next); err != nil {
		return fmt.Errorf("cannot read new shared.path")
	}
	oldPath, nextPath := resolvedSharedPointer(c.path, old), resolvedSharedPointer(c.path, next.Shared)
	if utils.PathsEqual(oldPath, nextPath) {
		return nil
	}
	roots := []string{}
	for index, raw := range [][]byte{previous, c.Bytes} {
		var transport struct {
			Source       string        `yaml:"source"`
			AgentsSource string        `yaml:"agents_source"`
			ExtrasSource string        `yaml:"extras_source"`
			Sources      GlobalSources `yaml:"sources"`
			GitRoot      string        `yaml:"git_root"`
		}
		if err := yaml.Unmarshal(raw, &transport); err != nil {
			// Only malformed OLD transport is unobservable.
			if index == 0 {
				continue
			}
			return fmt.Errorf("cannot read shared state publication boundary")
		}
		cfg := Config{Source: transport.Source, AgentsSource: transport.AgentsSource, ExtrasSource: transport.ExtrasSource, Sources: transport.Sources, GitRoot: transport.GitRoot}
		roots = append(roots, cfg.EffectiveGitRoot())
	}
	for _, p := range []string{oldPath, nextPath} {
		if p != "" {
			roots = append(roots, filepath.Dir(p))
		}
	}
	state, err := LoadSharedState(c.path, roots[0], "")
	if err != nil {
		return err
	}
	state.roots = roots
	if err := state.safePath(); err != nil {
		return err
	}
	state.RecordPointer(oldPath)
	state.RecordPointer(nextPath)
	before, err := state.stateCandidate()
	if err != nil {
		return err
	}
	destination, err := utils.ConfigWritePath(c.path, false)
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	previousCheck := c.check
	c.check = func() error {
		if previousCheck != nil {
			if err := previousCheck(); err != nil {
				return err
			}
		}
		current, err := utils.ConfigWritePath(c.path, false)
		if err != nil {
			return err
		}
		current, err = filepath.Abs(current)
		if err != nil {
			return err
		}
		if !utils.PathsEqual(destination, current) {
			return fmt.Errorf("config destination changed; retry the command")
		}
		raw, err := readConfigSnapshot(c.path)
		if err != nil {
			return err
		}
		if (raw == nil) != (previous == nil) || !bytes.Equal(raw, previous) {
			return fmt.Errorf("config changed before shared pointer replacement; retry the command")
		}
		return nil
	}
	// Preflight refuses unsafe or malformed state before any file is written.
	c.before = before
	return c.Check()
}
