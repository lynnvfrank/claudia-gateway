package bifrostadmin

import (
	"encoding/json"
	"testing"
)

func TestMergeProviderKey_roundTrip(t *testing.T) {
	in := []byte(`{"name":"groq","keys":[{"id":"k1","name":"x","weight":1,"value":{"value":"***"}}],"concurrency_and_buffer_size":{"concurrency":5,"buffer_size":10}}`)
	out, err := MergeProviderKey("groq", in, "new-secret")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	keys := doc["keys"].([]any)
	k0 := keys[0].(map[string]any)
	if k0["value"] != "new-secret" {
		t.Fatalf("%+v", k0)
	}
	cb := doc["concurrency_and_buffer_size"].(map[string]any)
	if cb["concurrency"].(float64) != 5 {
		t.Fatalf("%+v", cb)
	}
}

func TestMergeProviderKey_addsKey(t *testing.T) {
	in := []byte(`{"name":"groq","keys":[],"concurrency_and_buffer_size":{"concurrency":2,"buffer_size":3}}`)
	out, err := MergeProviderKey("groq", in, "abc")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	_ = json.Unmarshal(out, &doc)
	keys := doc["keys"].([]any)
	if len(keys) != 1 {
		t.Fatal(len(keys))
	}
	k0 := keys[0].(map[string]any)
	if k0["name"] != "claudia-ui-groq" {
		t.Fatalf("name: %v", k0["name"])
	}
}

func TestMergeProviderKey_emptyRoot(t *testing.T) {
	out, err := MergeProviderKey("gemini", []byte("{}"), "first-key")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	keys := doc["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("keys: %v", keys)
	}
	k0 := keys[0].(map[string]any)
	if k0["name"] != "claudia-ui-gemini" {
		t.Fatalf("name: %v", k0["name"])
	}
}

func TestMergeProviderKey_namesDifferByProvider(t *testing.T) {
	gq, err := MergeProviderKey("groq", []byte("{}"), "a")
	if err != nil {
		t.Fatal(err)
	}
	gm, err := MergeProviderKey("gemini", []byte("{}"), "b")
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	_ = json.Unmarshal(gq, &a)
	_ = json.Unmarshal(gm, &b)
	n0 := a["keys"].([]any)[0].(map[string]any)["name"]
	n1 := b["keys"].([]any)[0].(map[string]any)["name"]
	if n0 == n1 {
		t.Fatalf("both names %v", n0)
	}
}

func TestMergeOllamaBaseURL(t *testing.T) {
	in := []byte(`{"name":"ollama","keys":[],"network_config":{"base_url":"http://old:11434"},"concurrency_and_buffer_size":{"concurrency":1,"buffer_size":2}}`)
	out, err := MergeOllamaBaseURL(in, "http://host:11434")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	_ = json.Unmarshal(out, &doc)
	nc := doc["network_config"].(map[string]any)
	if nc["base_url"] != "http://host:11434" {
		t.Fatalf("%+v", nc)
	}
}

func TestMergeOllamaBaseURL_emptyRoot(t *testing.T) {
	out, err := MergeOllamaBaseURL([]byte("{}"), "http://host:11434")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	nc := doc["network_config"].(map[string]any)
	if nc["base_url"] != "http://host:11434" {
		t.Fatalf("%+v", nc)
	}
	keys := doc["keys"].([]any)
	if len(keys) != 0 {
		t.Fatalf("keys: %v", keys)
	}
}
