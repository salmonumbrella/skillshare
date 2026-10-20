package config

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// SaveCandidate holds the exact serialized bytes and the fresh, expanded view
// decoded from them. Err mirrors the error returned by PrepareSave. Preparation
// validates decoding and expansion; callers apply their own semantic validation.
type SaveCandidate struct {
	Bytes    []byte
	Expanded *Config
	Err      error
	path     string
}

// PrepareSave runs Save's serializer without changing the caller or filesystem.
// Existing comments, anchors and aliases participate in the effective result, so
// Expanded is decoded from Bytes rather than copied from next.
func PrepareSave(path string, next *Config) (*SaveCandidate, error) {
	c := &SaveCandidate{path: path}
	if next == nil {
		c.Err = fmt.Errorf("cannot prepare a nil config")
		return c, c.Err
	}
	payload := next.withoutProjectTargets()
	if next.PreserveTildeOnSave {
		payload = payload.cloneForSave()
	}
	data, err := marshalConfigFile(path, payload, schemaComment)
	if err != nil {
		c.Err = fmt.Errorf("failed to marshal config: %w", err)
		return c, c.Err
	}
	c.Bytes = data
	c.Expanded, c.Err = decodeConfig(path, data, false)
	return c, c.Err
}

// Write atomically persists the already prepared Bytes without serializing again.
// Resolving the destination keeps a symlinked config linked to its shared file.
func (c *SaveCandidate) Write() error {
	if c.Err != nil {
		return c.Err
	}
	dest, err := utils.ConfigWritePath(c.path, false)
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".skillshare-config-*")
	if err != nil {
		return fmt.Errorf("failed to create config temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())
	// Retain the existing config's permissions; new configs use Save's 0644.
	perm := os.FileMode(0644)
	if info, err := os.Stat(dest); err == nil {
		perm = info.Mode().Perm()
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to set config permissions: %w", err)
	}
	if _, err := tmp.Write(c.Bytes); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close config: %w", err)
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}
