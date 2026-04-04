// Desktop shell: Phase 5 message + optional live view of claudia GET /status (supervisor + gateway).
// Build with CGO; see docs/gui-testing.md. Run claudia or claudia serve separately; GUI polls HTTP only.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

// claudiaMessage is the required Phase 5 string (verbatim).
const claudiaMessage = "mew mew, Love Claudia"

func main() {
	gateway := flag.String("gateway", "http://127.0.0.1:3000", "Claudia gateway base URL (GET /status)")
	poll := flag.Duration("poll", 2*time.Second, "How often to refresh status")
	flag.Parse()

	base := strings.TrimSuffix(strings.TrimSpace(*gateway), "/")
	if base == "" {
		base = "http://127.0.0.1:3000"
	}

	a := app.NewWithID("com.lynn.claudia.gui")
	w := a.NewWindow("Claudia")
	statusStr := binding.NewString()
	_ = statusStr.Set("Connecting…")
	status := widget.NewLabelWithData(statusStr)
	status.Wrapping = fyne.TextWrapWord
	footer := widget.NewLabel(claudiaMessage)
	content := container.NewBorder(
		nil,
		container.NewCenter(footer),
		nil, nil,
		container.NewPadded(status),
	)
	w.SetContent(content)
	w.Resize(fyne.NewSize(520, 320))
	w.SetFixedSize(false)
	w.CenterOnScreen()

	client := &http.Client{Timeout: 4 * time.Second}
	go pollStatus(base, *poll, client, statusStr)

	w.ShowAndRun()
}

func pollStatus(base string, every time.Duration, client *http.Client, out binding.String) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	refresh := func() {
		text := fetchFormatted(client, base+"/status")
		_ = out.Set(text)
	}
	refresh()
	for range tick.C {
		refresh()
	}
}

func fetchFormatted(client *http.Client, statusURL string) string {
	req, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		return fmt.Sprintf("Request error: %v", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Offline — is claudia running?\n\n%v\n\nURL: %s", err, statusURL)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Sprintf("Read error: %v", err)
	}
	if res.StatusCode == http.StatusNotFound {
		return fmt.Sprintf("HTTP 404 — nothing at GET /status.\n\n"+
			"Rebuild and restart the gateway:\n"+
			"  go build -o claudia ./cmd/claudia\n\nURL: %s", statusURL)
	}
	bodyTrim := strings.TrimSpace(string(body))
	if !json.Valid(body) {
		return fmt.Sprintf("HTTP %d — body is not JSON (wrong port, or not the Go gateway?).\n\n%q\n\nURL: %s",
			res.StatusCode, truncate(bodyTrim, 280), statusURL)
	}
	var doc statusDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Sprintf("HTTP %d — could not parse /status JSON: %v\n\n%s",
			res.StatusCode, err, truncate(bodyTrim, 400))
	}
	return formatStatus(doc, res.StatusCode)
}

type statusDoc struct {
	Supervisor struct {
		Active           bool   `json:"active"`
		BifrostListen    string `json:"bifrost_listen"`
		QdrantSupervised bool   `json:"qdrant_supervised"`
		QdrantHTTP       string `json:"qdrant_http"`
	} `json:"supervisor"`
	Gateway struct {
		Listen       string `json:"listen"`
		VirtualModel string `json:"virtual_model"`
		Semver       string `json:"semver"`
		UpstreamURL  string `json:"upstream_base_url"`
	} `json:"gateway"`
	Upstream struct {
		HealthURL string `json:"health_url"`
		OK        bool   `json:"ok"`
		Status    int    `json:"status"`
		Detail    string `json:"detail"`
	} `json:"upstream"`
}

func formatStatus(d statusDoc, httpCode int) string {
	var b strings.Builder
	b.WriteString("Gateway status (GET /status)\n\n")
	if d.Supervisor.Active {
		b.WriteString("Supervisor: active (claudia serve)\n")
		if d.Supervisor.BifrostListen != "" {
			fmt.Fprintf(&b, "  BiFrost listen: %s\n", d.Supervisor.BifrostListen)
		}
		if d.Supervisor.QdrantSupervised {
			fmt.Fprintf(&b, "  Qdrant supervised: yes (%s)\n", d.Supervisor.QdrantHTTP)
		} else {
			b.WriteString("  Qdrant supervised: no\n")
		}
	} else {
		b.WriteString("Supervisor: not active (plain claudia gateway)\n")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "Listen: %s\n", d.Gateway.Listen)
	fmt.Fprintf(&b, "Virtual model: %s (semver %s)\n", d.Gateway.VirtualModel, d.Gateway.Semver)
	fmt.Fprintf(&b, "Upstream base: %s\n", d.Gateway.UpstreamURL)
	b.WriteString("\n")
	if d.Upstream.OK {
		fmt.Fprintf(&b, "Upstream probe: OK (HTTP %d)\n", d.Upstream.Status)
	} else {
		fmt.Fprintf(&b, "Upstream probe: degraded (HTTP %d)\n", d.Upstream.Status)
		if d.Upstream.Detail != "" {
			fmt.Fprintf(&b, "  detail: %s\n", d.Upstream.Detail)
		}
	}
	fmt.Fprintf(&b, "\nHTTP %d", httpCode)
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
