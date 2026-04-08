package bifrostadmin

import "testing"

func TestSummarizeProvider_plainKey(t *testing.T) {
	body := []byte(`{"name":"groq","keys":[{"value":{"value":"gsk_test1234567890"}}],"network_config":{}}`)
	s, err := SummarizeProvider("groq", body)
	if err != nil {
		t.Fatal(err)
	}
	if !s.KeyConfigured {
		t.Fatal("expected configured")
	}
	if s.KeyHint != "••••7890" {
		t.Fatalf("hint %q", s.KeyHint)
	}
}

func TestSummarizeProvider_env(t *testing.T) {
	body := []byte(`{"keys":[{"value":{"from_env":true,"env_var":"GROQ_API_KEY"}}]}`)
	s, err := SummarizeProvider("groq", body)
	if err != nil {
		t.Fatal(err)
	}
	if !s.KeyConfigured || s.KeyHint != "env:GROQ_API_KEY" {
		t.Fatalf("%+v", s)
	}
}

func TestSummarizeProvider_stringInlineKey(t *testing.T) {
	body := []byte(`{"name":"gemini","keys":[{"name":"x","value":"AIzaSyD6Kr4rtKFrxNGKCITsrmvuQd7EBhCJqIY"}]}`)
	s, err := SummarizeProvider("gemini", body)
	if err != nil {
		t.Fatal(err)
	}
	if !s.KeyConfigured {
		t.Fatal("expected configured")
	}
	if s.KeyHint != "••••JqIY" {
		t.Fatalf("hint %q", s.KeyHint)
	}
}

func TestSummarizeProvider_ollamaURL(t *testing.T) {
	body := []byte(`{"keys":[],"network_config":{"base_url":"http://localhost:11434"}}`)
	s, err := SummarizeProvider("ollama", body)
	if err != nil {
		t.Fatal(err)
	}
	if s.OllamaBaseURL != "http://localhost:11434" {
		t.Fatalf("%+v", s)
	}
}
