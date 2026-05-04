package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lynn/claudia-gateway/internal/indexer"
	"gopkg.in/yaml.v3"
)

const maxIndexerConfigYAMLBytes = 512 << 10

func indexerRootsJSON(fc indexer.FileConfig) []map[string]string {
	out := make([]map[string]string, 0, len(fc.Roots))
	for _, row := range fc.Roots {
		out = append(out, map[string]string{
			"path":         row.Path,
			"workspace_id": row.WorkspaceID,
			"project_id":   row.ProjectID,
			"flavor_id":    row.FlavorID,
		})
	}
	return out
}

func validateIndexerRootDir(rootPath string) (absRoot string, msg string, status int) {
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return "", "path required", http.StatusBadRequest
	}
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return "", err.Error(), http.StatusBadRequest
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return "", "path must be an existing directory", http.StatusBadRequest
	}
	return abs, "", 0
}

func (a *adminUI) handleIndexerConfigGET(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		http.Error(w, "no config", http.StatusInternalServerError)
		return
	}
	path := strings.TrimSpace(res.IndexerSupervisedConfigPath)
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "indexer supervised config path not configured"})
		return
	}
	if err := indexer.EnsureSupervisedConfigFile(path); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	var fc indexer.FileConfig
	rootsJSON := []map[string]string{}
	if err := yaml.Unmarshal(raw, &fc); err == nil {
		rootsJSON = indexerRootsJSON(fc)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"path":               path,
		"yaml":               string(raw),
		"roots":              rootsJSON,
		"supervised_enabled": res.IndexerSupervisedEnabled,
	})
}

func (a *adminUI) handleIndexerConfigPUT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		http.Error(w, "no config", http.StatusInternalServerError)
		return
	}
	path := strings.TrimSpace(res.IndexerSupervisedConfigPath)
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "indexer supervised config path not configured"})
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxIndexerConfigYAMLBytes+1<<12))
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := dec.Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid json"})
		return
	}
	y := strings.TrimSpace(body.YAML)
	if len(y) > maxIndexerConfigYAMLBytes {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "yaml too large"})
		return
	}
	if err := yaml.Unmarshal([]byte(y), new(indexer.FileConfig)); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("invalid indexer yaml: %v", err)})
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if err := os.WriteFile(path, []byte(y), 0o644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "path": path})
}

func (a *adminUI) handleIndexerAppendRootPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		http.Error(w, "no config", http.StatusInternalServerError)
		return
	}
	path := strings.TrimSpace(res.IndexerSupervisedConfigPath)
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "indexer supervised config path not configured"})
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	var body struct {
		Path        string `json:"path"`
		WorkspaceID string `json:"workspace_id"`
		ProjectID   string `json:"project_id"`
		FlavorID    string `json:"flavor_id"`
	}
	if err := dec.Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid json"})
		return
	}
	absRoot, msg, st := validateIndexerRootDir(body.Path)
	if st != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
		return
	}
	if err := indexer.EnsureSupervisedConfigFile(path); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	var fc indexer.FileConfig
	if err := yaml.Unmarshal(raw, &fc); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("parse config: %v", err)})
		return
	}
	row := indexer.RootYAML{
		Path:        absRoot,
		WorkspaceID: strings.TrimSpace(body.WorkspaceID),
		ProjectID:   strings.TrimSpace(body.ProjectID),
		FlavorID:    strings.TrimSpace(body.FlavorID),
	}
	fc.Roots = append(fc.Roots, row)
	out, err := yaml.Marshal(&fc)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":    true,
		"path":  path,
		"yaml":  string(out),
		"roots": indexerRootsJSON(fc),
	})
}

func (a *adminUI) handleIndexerRemoveRootPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		http.Error(w, "no config", http.StatusInternalServerError)
		return
	}
	path := strings.TrimSpace(res.IndexerSupervisedConfigPath)
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "indexer supervised config path not configured"})
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	var body struct {
		Index int `json:"index"`
	}
	if err := dec.Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid json"})
		return
	}
	if err := indexer.EnsureSupervisedConfigFile(path); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	var fc indexer.FileConfig
	if err := yaml.Unmarshal(raw, &fc); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("parse config: %v", err)})
		return
	}
	if body.Index < 0 || body.Index >= len(fc.Roots) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "root index out of range"})
		return
	}
	fc.Roots = append(fc.Roots[:body.Index], fc.Roots[body.Index+1:]...)
	out, err := yaml.Marshal(&fc)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":    true,
		"path":  path,
		"yaml":  string(out),
		"roots": indexerRootsJSON(fc),
	})
}

func (a *adminUI) handleIndexerUpdateRootPUT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		http.Error(w, "no config", http.StatusInternalServerError)
		return
	}
	path := strings.TrimSpace(res.IndexerSupervisedConfigPath)
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "indexer supervised config path not configured"})
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	var body struct {
		Index       int    `json:"index"`
		Path        string `json:"path"`
		WorkspaceID string `json:"workspace_id"`
		ProjectID   string `json:"project_id"`
		FlavorID    string `json:"flavor_id"`
	}
	if err := dec.Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid json"})
		return
	}
	absRoot, msg, st := validateIndexerRootDir(body.Path)
	if st != 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
		return
	}
	if err := indexer.EnsureSupervisedConfigFile(path); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	var fc indexer.FileConfig
	if err := yaml.Unmarshal(raw, &fc); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("parse config: %v", err)})
		return
	}
	if body.Index < 0 || body.Index >= len(fc.Roots) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "root index out of range"})
		return
	}
	fc.Roots[body.Index] = indexer.RootYAML{
		Path:        absRoot,
		WorkspaceID: strings.TrimSpace(body.WorkspaceID),
		ProjectID:   strings.TrimSpace(body.ProjectID),
		FlavorID:    strings.TrimSpace(body.FlavorID),
	}
	out, err := yaml.Marshal(&fc)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":    true,
		"path":  path,
		"yaml":  string(out),
		"roots": indexerRootsJSON(fc),
	})
}
