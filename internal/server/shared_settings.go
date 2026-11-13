package server

import (
	"errors"
	"net/http"
	"reflect"

	"skillshare/internal/config"
)

func writeSharedError(w http.ResponseWriter, err error) bool {
	var shared *config.SharedError
	if !errors.As(err, &shared) {
		return false
	}
	writeCodedError(w, http.StatusConflict, shared.Code, shared.Error(), map[string]string{"name": shared.Name, "path": shared.Path})
	return true
}

// Callers hold s.mu; project extras have no shared-file layer.
func (s *Server) requireLocalExtra(w http.ResponseWriter, names ...string) bool {
	if s.IsProjectMode() {
		return true
	}
	for _, name := range names {
		if writeSharedError(w, s.cfg.CheckExtraWritable(name)) {
			return false
		}
	}
	return true
}

// Validate the entire assignment batch before its first restore or copy.
func (s *Server) checkSharedExtraChanges(planned []config.ExtraConfig) error {
	if s.IsProjectMode() {
		return nil
	}
	next := make(map[string]config.ExtraConfig, len(planned))
	for _, extra := range planned {
		next[extra.Name] = extra
	}
	for _, extra := range s.cfg.Extras {
		if candidate, ok := next[extra.Name]; !ok || !reflect.DeepEqual(extra, candidate) {
			if err := s.cfg.CheckExtraWritable(extra.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) sharedStatusPayload() *config.SharedStatus {
	if s.IsProjectMode() || !s.cfg.SharedEnabled() {
		return nil
	}
	status := s.cfg.SharedStatus()
	return &status
}
