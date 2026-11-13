package server

import (
	"errors"
	"net/http"
	"os"
	"slices"
	"time"

	"skillshare/internal/config"
)

// targetFile is one extra file a target's tool reads (see config.TargetFiles).
type targetFile struct {
	Path       string `json:"path"`
	Abs        string `json:"abs"`
	Builtin    bool   `json:"builtin"`
	Exists     bool   `json:"exists"`
	Size       int64  `json:"size"`
	LinkTo     string `json:"link_to,omitempty"`     // symlink destination of the file
	LinkShared string `json:"link_shared,omitempty"` // shared file (single-file extra) that link belongs to
}

type targetFileContent struct {
	targetFile
	Content string `json:"content"`
}

type targetFilesResponse struct {
	Target  string       `json:"target"`
	Project bool         `json:"project"`
	Root    string       `json:"root"`
	Files   []targetFile `json:"files"`
}

// targetFileScope is a configured target with its file root and files.
type targetFileScope struct {
	name  string
	tc    config.TargetConfig
	root  string // "" when the target has no file root
	files []config.TargetFile
}

// targetFileScope looks up a configured target, writing a 404 and reporting
// false when there is none. Callers must hold s.mu.
func (s *Server) targetFileScope(w http.ResponseWriter, name string) (targetFileScope, bool) {
	tc, found := s.cfg.Targets[name]
	if !found {
		writeCodedError(w, http.StatusNotFound, "target_files_not_found", "target not found: "+name, map[string]string{"target": name})
		return targetFileScope{}, false
	}
	sc := targetFileScope{name: name, tc: tc}
	if root, ok := config.TargetFileRoot(name, tc, s.IsProjectMode(), s.projectRoot); ok {
		sc.root = root
		sc.files = config.TargetFiles(name, tc, s.IsProjectMode())
	}
	return sc, true
}

// find returns the listed file with the given (uncleaned) path.
func (sc targetFileScope) find(rel string) (config.TargetFile, bool) {
	rel = config.CleanTargetFilePath(rel)
	for _, f := range sc.files {
		if f.Path == rel {
			return f, true
		}
	}
	return config.TargetFile{}, false
}

// describeTargetFile reports a file's state on disk. Callers must hold s.mu.
func (s *Server) describeTargetFile(f config.TargetFile, abs string) targetFile {
	out := targetFile{Path: f.Path, Abs: abs, Builtin: f.Builtin}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		out.Exists, out.Size = true, info.Size()
	}
	if dest, err := os.Readlink(abs); err == nil {
		out.LinkTo = dest
		out.LinkShared = s.sharedLinkName(abs)
	}
	return out
}

// targetFilesList builds the list response. Listed paths that no longer pass
// validation (edited by hand) are left out. Callers must hold s.mu.
func (s *Server) targetFilesList(sc targetFileScope) targetFilesResponse {
	resp := targetFilesResponse{Target: sc.name, Project: s.IsProjectMode(), Root: sc.root, Files: []targetFile{}}
	for _, f := range sc.files {
		if abs, err := config.ValidateTargetFilePath(sc.root, f.Path); err == nil {
			resp.Files = append(resp.Files, s.describeTargetFile(f, abs))
		}
	}
	return resp
}

func targetFileReason(err error) string {
	var pe *config.TargetFilePathError
	if errors.As(err, &pe) {
		return pe.Reason
	}
	return config.TargetFileOutside
}

func writeTargetFileInvalid(w http.ResponseWriter, rel, reason string) {
	writeCodedError(w, http.StatusBadRequest, "target_file_invalid_path", "invalid file path "+rel+": "+reason, map[string]string{"reason": reason})
}

func writeTargetFileNotListed(w http.ResponseWriter, name, rel string) {
	writeCodedError(w, http.StatusNotFound, "target_file_not_listed", rel+" is not a file of "+name, map[string]string{"target": name, "path": rel})
}

// setTargetUserFiles replaces the user's file list of a target in the config
// of the current mode. Callers must hold s.mu and save the config afterwards.
func (s *Server) setTargetUserFiles(name string, files []string) {
	tc := s.cfg.Targets[name]
	tc.Files = files
	s.cfg.Targets[name] = tc
	if !s.IsProjectMode() {
		return
	}
	for i := range s.projectCfg.Targets {
		if s.projectCfg.Targets[i].Name == name {
			s.projectCfg.Targets[i].Files = files
			return
		}
	}
}

// handleListTargetFiles — GET /api/targets/{name}/files
func (s *Server) handleListTargetFiles(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.targetFileScope(w, r.PathValue("name"))
	if !ok {
		return
	}
	writeJSON(w, s.targetFilesList(sc))
}

// listedTargetFile resolves a listed file for the content routes, writing the
// error and reporting false when it is not usable. Callers must hold s.mu.
func (s *Server) listedTargetFile(w http.ResponseWriter, sc targetFileScope, rel string) (config.TargetFile, string, bool) {
	f, listed := sc.find(rel)
	if !listed {
		writeTargetFileNotListed(w, sc.name, rel)
		return config.TargetFile{}, "", false
	}
	abs, err := config.ValidateTargetFilePath(sc.root, f.Path)
	if err != nil {
		writeTargetFileInvalid(w, rel, targetFileReason(err))
		return config.TargetFile{}, "", false
	}
	return f, abs, true
}

// targetFileContentResponse reads the file for a content response.
func (s *Server) targetFileContentResponse(f config.TargetFile, abs string) (targetFileContent, error) {
	resp := targetFileContent{targetFile: s.describeTargetFile(f, abs)}
	if resp.Exists {
		data, err := readLimited(abs)
		if err != nil {
			return resp, err
		}
		resp.Content = string(data)
	}
	return resp, nil
}

// handleGetTargetFileContent — GET /api/targets/{name}/files/content?path=
func (s *Server) handleGetTargetFileContent(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.targetFileScope(w, r.PathValue("name"))
	if !ok {
		return
	}
	f, abs, ok := s.listedTargetFile(w, sc, rel)
	if !ok {
		return
	}
	resp, err := s.targetFileContentResponse(f, abs)
	if err != nil {
		writeCodedError(w, http.StatusInternalServerError, "target_file_read_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, resp)
}

// handlePutTargetFileContent — PUT /api/targets/{name}/files/content?path=
// Writes the file, creating its folders; a symlinked file is written through.
func (s *Server) handlePutTargetFileContent(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name, rel := r.PathValue("name"), r.URL.Query().Get("path")
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, &body, maxInstructionsBytes+4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "target_file_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.targetFileScope(w, name)
	if !ok {
		return
	}
	f, abs, ok := s.listedTargetFile(w, sc, rel)
	if !ok {
		return
	}
	args := map[string]any{"target": name, "path": f.Path, "scope": "ui"}
	if err := writeInstructionsFile(abs, body.Content); err != nil {
		s.writeOpsLog("target-file-edit", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "target_file_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("target-file-edit", "ok", start, args, "")
	resp, err := s.targetFileContentResponse(f, abs)
	if err != nil {
		writeCodedError(w, http.StatusInternalServerError, "target_file_read_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, resp)
}

// handleAddTargetFile — POST /api/targets/{name}/files
// Adds a file to the target's list in the config of the current mode.
func (s *Server) handleAddTargetFile(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "target_file_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.targetFileScope(w, name)
	if !ok {
		return
	}
	if _, err := config.ValidateTargetFilePath(sc.root, body.Path); err != nil {
		writeTargetFileInvalid(w, body.Path, targetFileReason(err))
		return
	}
	if _, listed := sc.find(body.Path); listed {
		writeTargetFileInvalid(w, body.Path, "listed")
		return
	}
	rel := config.CleanTargetFilePath(body.Path)
	s.setTargetUserFiles(name, append(slices.Clone(sc.tc.Files), rel))
	args := map[string]any{"target": name, "path": rel, "scope": "ui"}
	if err := s.saveAndReloadConfig(); err != nil {
		s.writeOpsLog("target-file-add", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "target_file_save_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("target-file-add", "ok", start, args, "")
	s.writeTargetFilesAfterSave(w, name)
}

// handleRemoveTargetFile — DELETE /api/targets/{name}/files?path=
// Removes a file from the user's list; the file on disk is left alone.
func (s *Server) handleRemoveTargetFile(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name, rel := r.PathValue("name"), r.URL.Query().Get("path")
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.targetFileScope(w, name)
	if !ok {
		return
	}
	clean := config.CleanTargetFilePath(rel)
	if f, listed := sc.find(rel); listed && f.Builtin {
		writeCodedError(w, http.StatusBadRequest, "target_file_builtin", clean+" is a built-in file of "+name, map[string]string{"target": name, "path": clean})
		return
	}
	i := slices.IndexFunc(sc.tc.Files, func(p string) bool { return config.CleanTargetFilePath(p) == clean })
	if clean == "" || i == -1 {
		writeTargetFileNotListed(w, name, rel)
		return
	}
	files := slices.DeleteFunc(slices.Clone(sc.tc.Files), func(p string) bool { return config.CleanTargetFilePath(p) == clean })
	if len(files) == 0 {
		files = nil
	}
	s.setTargetUserFiles(name, files)
	args := map[string]any{"target": name, "path": clean, "scope": "ui"}
	if err := s.saveAndReloadConfig(); err != nil {
		s.writeOpsLog("target-file-remove", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "target_file_save_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("target-file-remove", "ok", start, args, "")
	s.writeTargetFilesAfterSave(w, name)
}

// writeTargetFilesAfterSave writes the list response from the reloaded
// config. Callers must hold s.mu.
func (s *Server) writeTargetFilesAfterSave(w http.ResponseWriter, name string) {
	sc, ok := s.targetFileScope(w, name)
	if !ok {
		return
	}
	writeJSON(w, s.targetFilesList(sc))
}
