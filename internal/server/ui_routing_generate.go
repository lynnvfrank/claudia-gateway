package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/routing"
	"github.com/lynn/claudia-gateway/internal/routinggen"
	"github.com/lynn/claudia-gateway/internal/upstream"
	"gopkg.in/yaml.v3"
)

type routingDraft struct {
	IDs                []string
	Pool               []string
	Chain              []string
	RouteYAML          []byte
	FilterFreeTierFlag bool
}

func summarizeRoutingYAML(b []byte) map[string]any {
	var doc struct {
		AmbiguousDefault string `yaml:"ambiguous_default_model"`
		Rules            []struct {
			Name string `yaml:"name"`
			When struct {
				Min *int `yaml:"min_message_chars"`
			} `yaml:"when"`
			Models []string `yaml:"models"`
		} `yaml:"rules"`
	}
	_ = yaml.Unmarshal(b, &doc)
	ruleOut := make([]map[string]any, 0, len(doc.Rules))
	for _, r := range doc.Rules {
		init := ""
		if len(r.Models) > 0 {
			init = r.Models[0]
		}
		entry := map[string]any{
			"name":          r.Name,
			"initial_model": init,
		}
		if r.When.Min != nil {
			entry["min_message_chars"] = *r.When.Min
		}
		ruleOut = append(ruleOut, entry)
	}
	return map[string]any{
		"ambiguous_default_model": doc.AmbiguousDefault,
		"rules":                   ruleOut,
	}
}

func (a *adminUI) computeRoutingDraft(ctx context.Context, res *config.Resolved) (*routingDraft, int, map[string]any) {
	apiKey := a.rt.UpstreamAPIKey()
	if apiKey == "" {
		return nil, http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{"message": "missing upstream API key", "type": "gateway_config"},
		}
	}
	timeout := healthTimeout(res)
	ctx, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()
	st, body, ok := upstream.FetchOpenAIModels(ctx, res.UpstreamBaseURL, apiKey, timeout, a.log)
	if !ok {
		return nil, http.StatusBadGateway, map[string]any{
			"error": map[string]any{
				"message": "Failed to list models from upstream",
				"type":    "gateway_upstream",
				"status":  st,
			},
		}
	}
	ids, err := routinggen.ExtractCatalogModelIDs(body, res.VirtualModelID)
	if err != nil {
		return nil, http.StatusBadGateway, map[string]any{
			"error": map[string]any{"message": "invalid upstream models JSON", "type": "gateway_upstream"},
		}
	}
	pool := ids
	if res.FilterFreeTierModels {
		if res.ProviderFreeTierSpec == nil || res.ProviderFreeTierSpec.Empty() {
			return nil, http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "routing.filter_free_tier_models is true but provider-free-tier.yaml is missing, invalid, or empty",
					"type":    "gateway_config",
				},
			}
		}
		pool = res.ProviderFreeTierSpec.Filter(ids)
	}
	if len(pool) == 0 {
		return nil, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "no models left after catalog and optional free-tier filter",
				"type":    "gateway_config",
			},
		}
	}
	chain := routinggen.OrderFallbackChain(pool)
	routeYAML, err := routinggen.BuildRoutingPolicyYAML(chain)
	if err != nil {
		return nil, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "gateway_config"},
		}
	}
	if err := routing.ValidatePolicyYAML(routeYAML); err != nil {
		return nil, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "generated routing policy failed validation: " + err.Error(),
				"type":    "gateway_config",
			},
		}
	}
	return &routingDraft{
		IDs: ids, Pool: pool, Chain: chain, RouteYAML: routeYAML,
		FilterFreeTierFlag: res.FilterFreeTierModels,
	}, 0, nil
}

func (a *adminUI) routingDraftResponse(d *routingDraft, saved bool) map[string]any {
	out := map[string]any{
		"ok":                           true,
		"saved":                        saved,
		"fallback_chain":               d.Chain,
		"models_upstream":              len(d.IDs),
		"models_used":                  len(d.Pool),
		"routing_policy_yaml":          string(d.RouteYAML),
		"routing":                      summarizeRoutingYAML(d.RouteYAML),
		"filter_free_tier_models_flag": d.FilterFreeTierFlag,
	}
	return out
}

func (a *adminUI) handleRoutingPreviewPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "gateway not configured")
		return
	}
	draft, st, errObj := a.computeRoutingDraft(r.Context(), res)
	if errObj != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		_ = json.NewEncoder(w).Encode(errObj)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a.routingDraftResponse(draft, false))
}

func (a *adminUI) handleRoutingGeneratePOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "gateway not configured")
		return
	}
	draft, st, errObj := a.computeRoutingDraft(r.Context(), res)
	if errObj != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(st)
		_ = json.NewEncoder(w).Encode(errObj)
		return
	}

	gwRaw, err := os.ReadFile(res.GatewayYAMLPath)
	if err != nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "read gateway.yaml")
		return
	}
	gwPatched, err := config.PatchGatewayYAMLBytesWithFallbackChain(gwRaw, draft.Chain)
	if err != nil {
		writeRoutingGenJSONError(w, http.StatusBadRequest, "gateway.yaml patch validation failed: "+err.Error())
		return
	}
	tmpValidate, err := os.CreateTemp(filepath.Dir(res.GatewayYAMLPath), "claudia-gw-validate-*.yaml")
	if err != nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "temp file")
		return
	}
	tmpPath := tmpValidate.Name()
	_ = tmpValidate.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := os.WriteFile(tmpPath, gwPatched, 0o600); err != nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "stage gateway validate")
		return
	}
	if _, err := config.LoadGatewayYAML(tmpPath, nil); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "gateway.yaml after patch failed to load: " + err.Error(),
				"type":    "gateway_config",
			},
		})
		return
	}

	routePerm := fs.FileMode(0o644)
	if st, err := os.Stat(res.RoutingPolicyPath); err == nil {
		routePerm = st.Mode() & fs.ModePerm
	}
	gwPerm := fs.FileMode(0o644)
	if st, err := os.Stat(res.GatewayYAMLPath); err == nil {
		gwPerm = st.Mode() & fs.ModePerm
	}

	if err := config.CommitRoutingAndGateway(res.RoutingPolicyPath, draft.RouteYAML, routePerm, res.GatewayYAMLPath, gwPatched, gwPerm); err != nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rb, rerr := os.ReadFile(res.RoutingPolicyPath)
	if rerr != nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "read back routing-policy.yaml")
		return
	}
	if err := routing.ValidatePolicyYAML(rb); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "routing policy on disk failed validation after write",
				"type":    "gateway_config",
				"detail":  err.Error(),
			},
		})
		return
	}
	if _, err := config.LoadGatewayYAML(res.GatewayYAMLPath, nil); err != nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "reload gateway.yaml after write: "+err.Error())
		return
	}

	a.rt.Sync()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a.routingDraftResponse(draft, true))
}

func (a *adminUI) handleRoutingEvaluatePOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "gateway not configured")
		return
	}

	var body struct {
		RoutingPolicyYAML string          `json:"routing_policy_yaml"`
		FallbackChain     []string        `json:"fallback_chain"`
		VirtualModelID    string          `json:"virtual_model_id"`
		Messages          json.RawMessage `json:"messages"`
		SmokeCompletion   bool            `json:"smoke_completion"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
	if err := dec.Decode(&body); err != nil {
		writeRoutingGenJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	yamlStr := strings.TrimSpace(body.RoutingPolicyYAML)
	if yamlStr == "" {
		writeRoutingGenJSONError(w, http.StatusBadRequest, "routing_policy_yaml required")
		return
	}
	if len(body.FallbackChain) == 0 {
		writeRoutingGenJSONError(w, http.StatusBadRequest, "fallback_chain required")
		return
	}
	vm := strings.TrimSpace(body.VirtualModelID)
	if vm == "" {
		vm = res.VirtualModelID
	}
	policyBytes := []byte(yamlStr)

	rawMsgs := body.Messages
	if len(rawMsgs) == 0 {
		rawMsgs = json.RawMessage(`[{"role":"user","content":"Hello."}]`)
	}
	modelField, err := json.Marshal(vm)
	if err != nil {
		writeRoutingGenJSONError(w, http.StatusBadRequest, "virtual model id")
		return
	}
	reqMap := map[string]json.RawMessage{
		"model":    modelField,
		"messages": rawMsgs,
	}
	initial, via, err := routing.EvaluatePick(policyBytes, reqMap, body.FallbackChain, vm, a.log)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": err.Error(), "type": "gateway_config"},
		})
		return
	}
	idx := routing.StartingFallbackIndex(initial, body.FallbackChain)
	slice := append([]string(nil), body.FallbackChain[idx:]...)

	out := map[string]any{
		"ok":                    true,
		"initial_model":         initial,
		"via":                   string(via),
		"fallback_start_index":  idx,
		"fallback_from_initial": slice,
	}

	if body.SmokeCompletion {
		apiKey := a.rt.UpstreamAPIKey()
		if apiKey == "" {
			out["smoke_completion"] = map[string]any{"ok": false, "error": "missing upstream API key"}
		} else if initial == "" {
			out["smoke_completion"] = map[string]any{"ok": false, "error": "no initial model to probe"}
		} else {
			to := healthTimeout(res)
			if to > 45*time.Second {
				to = 45 * time.Second
			}
			ctx, cancel := context.WithTimeout(r.Context(), to+2*time.Second)
			defer cancel()
			st, ok, det := upstream.SmokeChatCompletion(ctx, res.UpstreamBaseURL, apiKey, initial, to, a.log)
			sm := map[string]any{"ok": ok, "status": st, "detail": det}
			out["smoke_completion"] = sm
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (a *adminUI) handleRoutingFilterFreeTierPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14))
	if err := dec.Decode(&body); err != nil {
		writeRoutingGenJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		writeRoutingGenJSONError(w, http.StatusInternalServerError, "gateway not configured")
		return
	}
	if err := config.WriteGatewayFilterFreeTierModels(res.GatewayYAMLPath, body.Enabled); err != nil {
		writeRoutingGenJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.rt.Sync()
	res2, _, _ := a.rt.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                      true,
		"filter_free_tier_models": res2.FilterFreeTierModels,
	})
}

func writeRoutingGenJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": "gateway_config"},
	})
}
