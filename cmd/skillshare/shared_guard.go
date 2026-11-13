package main

import (
	"skillshare/internal/config"
	"skillshare/internal/install"
)

func guardSharedWorktree(cfg *config.Config, source string, dryRun bool) error {
	if err := cfg.CheckSharedWorktree(source); err != nil {
		return err
	}
	return cfg.RecordSharedPointer(source, dryRun)
}
func guardSharedOutgoing(cfg *config.Config, source string, opts install.ParseOptions) error {
	return cfg.CheckSharedOutgoing(source, opts)
}
