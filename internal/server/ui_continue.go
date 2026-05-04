package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lynn/claudia-gateway/internal/indexer"
	"gopkg.in/yaml.v3"
)

const maxContinueAPIBody = 1 << 14

type continueDoc struct {
	Name    string          `yaml:"name"`
	Version string          `yaml:"version"`
	Schema  string          `yaml:"schema"`
	Models  []continueModel `yaml:"models"`
}

type continueModel struct {
	Name           string              `yaml:"name"`
	Model          string              `yaml:"model"`
	Provider       string              `yaml:"provider"`
	APIKey         string              `yaml:"apiKey"`
	APIBase        string              `yaml:"apiBase"`
	RequestOptions *continueReqOptions `yaml:"requestOptions,omitempty"`
	Roles          []string            `yaml:"roles"`
	Capabilities   []string            `yaml:"capabilities,omitempty"`
}

type continueHeadersYAML struct {
	Project string `yaml:"X-Claudia-Project,omitempty"`
	Flavor  string `yaml:"X-Claudia-Flavor-Id,omitempty"`
}

type continueReqOptions struct {
	Headers continueHeadersYAML `yaml:"headers"`
}

func ingestProjectForContinue(projectID, workspaceID string) string {
	return indexer.IngestProject(indexer.ScopeFragment{
		ProjectID:   strings.TrimSpace(projectID),
		WorkspaceID: strings.TrimSpace(workspaceID),
	})
}

func continueConfigYAMLBytes(semver, virtualModel, publicBase, token, projectID, workspaceID, flavor string) ([]byte, error) {
	base := strings.TrimSuffix(strings.TrimSpace(publicBase), "/")
	if base == "" {
		base = "http://127.0.0.1:3000"
	}
	apiBase := base + "/v1"
	ver := semver
	if ver == "" {
		ver = "0.1.0"
	}
	vm := strings.TrimSpace(virtualModel)
	if vm == "" {
		vm = "Claudia-0.1.0"
	}
	h := continueHeadersYAML{}
	if p := ingestProjectForContinue(projectID, workspaceID); p != "" {
		h.Project = p
	}
	if f := strings.TrimSpace(flavor); f != "" {
		h.Flavor = f
	}
	var ro *continueReqOptions
	if h.Project != "" || h.Flavor != "" {
		ro = &continueReqOptions{Headers: h}
	}
	doc := continueDoc{
		Name:    "Claudia",
		Version: ver,
		Schema:  "v1",
		Models: []continueModel{{
			Name:           "Cℓαudια",
			Model:          vm,
			Provider:       "openai",
			APIKey:         token,
			APIBase:        apiBase,
			RequestOptions: ro,
			Roles:          []string{"chat", "edit", "apply"},
			Capabilities:   []string{"reasoning", "tool_use"},
		}},
	}
	var buf bytes.Buffer
	if _, err := buf.WriteString("%YAML 1.1\n---\n"); err != nil {
		return nil, err
	}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type supervisedRootRow struct {
	AbsPath     string
	ProjectID   string
	WorkspaceID string
	FlavorID    string
}

// loadSupervisedIndexerRoots returns absolute path per root (for authz) and scope fields.
func (a *adminUI) loadSupervisedIndexerRoots() ([]supervisedRootRow, string, error) {
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		return nil, "", fmt.Errorf("no config")
	}
	cfgPath := strings.TrimSpace(res.IndexerSupervisedConfigPath)
	if cfgPath == "" {
		return nil, "", fmt.Errorf("indexer supervised config path not configured")
	}
	if err := indexer.EnsureSupervisedConfigFile(cfgPath); err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, "", err
	}
	var fc indexer.FileConfig
	if err := yaml.Unmarshal(raw, &fc); err != nil {
		return nil, "", err
	}
	out := make([]supervisedRootRow, 0, len(fc.Roots))
	for _, row := range fc.Roots {
		p := strings.TrimSpace(row.Path)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		out = append(out, supervisedRootRow{
			AbsPath:     filepath.Clean(abs),
			ProjectID:   row.ProjectID,
			WorkspaceID: row.WorkspaceID,
			FlavorID:    row.FlavorID,
		})
	}
	return out, cfgPath, nil
}

func (a *adminUI) findSupervisedRoot(absWant string) (*supervisedRootRow, bool) {
	want := filepath.Clean(absWant)
	roots, _, err := a.loadSupervisedIndexerRoots()
	if err != nil {
		return nil, false
	}
	for i := range roots {
		if roots[i].AbsPath == want {
			return &roots[i], true
		}
	}
	return nil, false
}

func writeContinueJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (a *adminUI) handleContinueFileStatusPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxContinueAPIBody))
	var body struct {
		RootPath string `json:"root_path"`
	}
	if err := dec.Decode(&body); err != nil {
		writeContinueJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	absRoot, msg, st := validateIndexerRootDir(body.RootPath)
	if st != 0 {
		writeContinueJSONError(w, st, msg)
		return
	}
	if _, ok := a.findSupervisedRoot(absRoot); !ok {
		writeContinueJSONError(w, http.StatusBadRequest, "path is not a configured indexer root")
		return
	}
	contDir := filepath.Join(absRoot, ".continue")
	cfgPath := filepath.Join(contDir, "config.yaml")
	_, err := os.Stat(cfgPath)
	exists := err == nil
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"exists": exists,
		"path":   cfgPath,
	})
}

func (a *adminUI) handleContinueWritePOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxContinueAPIBody))
	var body struct {
		RootPath string `json:"root_path"`
	}
	if err := dec.Decode(&body); err != nil {
		writeContinueJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	tok := ""
	if c, err := r.Cookie(a.cookieName()); err == nil && c.Value != "" {
		tok = a.opts.Sessions.GatewayToken(c.Value)
	}
	if strings.TrimSpace(tok) == "" {
		writeContinueJSONError(w, http.StatusUnauthorized, "no gateway token in session; sign in again")
		return
	}
	absRoot, msg, st := validateIndexerRootDir(body.RootPath)
	if st != 0 {
		writeContinueJSONError(w, st, msg)
		return
	}
	row, ok := a.findSupervisedRoot(absRoot)
	if !ok {
		writeContinueJSONError(w, http.StatusBadRequest, "path is not a configured indexer root")
		return
	}
	contDir := filepath.Join(absRoot, ".continue")
	cfgPath := filepath.Join(contDir, "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		writeContinueJSONError(w, http.StatusConflict, "config file already exists")
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		writeContinueJSONError(w, http.StatusInternalServerError, "no config")
		return
	}
	semver := res.Semver
	vm := res.VirtualModelID
	pub := publicGatewayBase(r)
	raw, err := continueConfigYAMLBytes(semver, vm, pub, tok, row.ProjectID, row.WorkspaceID, row.FlavorID)
	if err != nil {
		writeContinueJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.MkdirAll(contDir, 0o755); err != nil {
		writeContinueJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		writeContinueJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "path": cfgPath})
}
