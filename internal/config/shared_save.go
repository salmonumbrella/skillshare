package config

import (
	"os"
	"path/filepath"
	"reflect"

	"skillshare/internal/sharedfile"
	"skillshare/internal/utils"
)

// sharedSavePayload owns the strip/refuse invariant for every serializer.
// payload must already be a private top-level copy of the caller.
func sharedSavePayload(payload *Config) error {
	layer := payload.sharedLayer
	if layer != nil {
		for name, baseline := range layer.baseline {
			count := 0
			for _, entry := range payload.Extras {
				if entry.Name == name {
					count++
					if !reflect.DeepEqual(entry, baseline) {
						return sharedOwnedSaveError(layer, name)
					}
				}
			}
			if count != 1 {
				return sharedOwnedSaveError(layer, name)
			}
		}
		var local []ExtraConfig
		for _, entry := range payload.Extras {
			if layer.origins[entry.Name] != sharedfile.Shared {
				local = append(local, entry)
			}
		}
		payload.Extras = local
	}
	if payload.Shared != nil {
		pointer := *payload.Shared
		if home, err := os.UserHomeDir(); err == nil {
			pointer.Path = filepath.ToSlash(utils.FoldHomePathWith(filepath.FromSlash(pointer.Path), filepath.FromSlash(home)))
		}
		payload.Shared = &pointer
	}
	return nil
}
func sharedOwnedSaveError(layer *sharedLayer, name string) error {
	return &SharedError{Code: SharedOwned, Kind: "extras", Name: name, Path: layer.status.Path}
}
