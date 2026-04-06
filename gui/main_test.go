package main

import (
	"strings"
	"testing"
)

func TestPhase5Message(t *testing.T) {
	const want = "mew mew, Love Claudia"
	if claudiaMessage != want {
		t.Fatalf("got %q want %q", claudiaMessage, want)
	}
}

func TestFormatStatus_supervisorActive(t *testing.T) {
	var d statusDoc
	d.Supervisor.Active = true
	d.Supervisor.BifrostListen = "127.0.0.1:8080"
	d.Gateway.Listen = ":3000"
	d.Gateway.VirtualModel = "Claudia-0.1.0"
	d.Gateway.Semver = "0.1.0"
	d.Upstream.OK = true
	d.Upstream.Status = 200
	out := formatStatus(d, 200)
	if !strings.Contains(out, "Supervisor: active") || !strings.Contains(out, "BiFrost listen") {
		t.Fatalf("%q", out)
	}
}
