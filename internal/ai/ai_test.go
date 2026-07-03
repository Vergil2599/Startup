package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openAIMock(t *testing.T, wantAuth string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if wantAuth != "" && r.Header.Get("Authorization") != wantAuth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct{ Role, Content string }
		}
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "eco: " + req.Messages[len(req.Messages)-1].Content + " [" + req.Model + "]"}}},
			"usage":   map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
}

func TestOpenAICompatComplete(t *testing.T) {
	srv := openAIMock(t, "Bearer sk-test")
	defer srv.Close()
	p := NewOpenAICompat("test", srv.URL, "sk-test", "gpt-x", 5*time.Second)
	resp, err := p.Complete(context.Background(), Request{Prompt: "hola"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "eco: hola") || !strings.Contains(resp.Text, "gpt-x") {
		t.Fatalf("text=%q (debe usar el modelo por defecto)", resp.Text)
	}
	if resp.TokensIn != 10 || resp.TokensOut != 5 {
		t.Fatalf("tokens: %d/%d", resp.TokensIn, resp.TokensOut)
	}
	// Override de modelo por petición.
	resp, _ = p.Complete(context.Background(), Request{Prompt: "x", Model: "otro"})
	if !strings.Contains(resp.Text, "[otro]") {
		t.Fatalf("override de modelo: %q", resp.Text)
	}
}

func TestOpenAICompatAuthError(t *testing.T) {
	srv := openAIMock(t, "Bearer correcto")
	defer srv.Close()
	p := NewOpenAICompat("test", srv.URL, "incorrecto", "m", 5*time.Second)
	if _, err := p.Complete(context.Background(), Request{Prompt: "x"}); err == nil ||
		!strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("esperaba HTTP 401: %v", err)
	}
}

func TestOllamaComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
			Stream bool   `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			t.Error("v1 debe pedir stream=false")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"response": "ollama dice: " + req.Prompt, "prompt_eval_count": 7, "eval_count": 3,
		})
	}))
	defer srv.Close()
	p := NewOllama("local", srv.URL, "llama3", 5*time.Second)
	resp, err := p.Complete(context.Background(), Request{Prompt: "hola"})
	if err != nil || !strings.Contains(resp.Text, "ollama dice: hola") {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if resp.TokensIn != 7 || resp.TokensOut != 3 {
		t.Fatalf("tokens: %d/%d", resp.TokensIn, resp.TokensOut)
	}
}

func TestCancellationPropagates(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	defer srv.Close()
	defer close(blocked)
	p := NewOllama("lento", srv.URL, "m", 30*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := p.Complete(ctx, Request{Prompt: "x"}); err == nil {
		t.Fatal("la cancelación debe propagarse")
	}
}

func TestLoadConfigs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".pes"), 0o755)

	// Sin archivo: lista vacía, sin error (offline puro).
	cfgs, err := LoadConfigs(root)
	if err != nil || cfgs != nil {
		t.Fatalf("cfgs=%v err=%v", cfgs, err)
	}

	good := `providers:
  - {name: local, type: ollama, base_url: "http://localhost:11434", model: llama3}
  - {name: lmstudio, type: openai, base_url: "http://localhost:1234", model: qwen, api_key_env: LM_KEY}
`
	os.WriteFile(filepath.Join(root, ".pes", "providers.yaml"), []byte(good), 0o644)
	cfgs, err = LoadConfigs(root)
	if err != nil || len(cfgs) != 2 {
		t.Fatalf("cfgs=%v err=%v", cfgs, err)
	}

	for _, bad := range []string{
		"providers:\n  - {name: x, type: magia, base_url: h}",
		"providers:\n  - {type: ollama, base_url: h}",
		"{{{",
	} {
		os.WriteFile(filepath.Join(root, ".pes", "providers.yaml"), []byte(bad), 0o644)
		if _, err := LoadConfigs(root); err == nil {
			t.Errorf("config inválida aceptada: %q", bad)
		}
	}
}

func TestBuildReadsKeyFromEnv(t *testing.T) {
	t.Setenv("TEST_PES_KEY", "sk-desde-env")
	p, err := Build(ProviderConfig{Name: "x", Type: "openai", BaseURL: "http://h", Model: "m", APIKeyEnv: "TEST_PES_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	if oc := p.(*OpenAICompat); oc.apiKey != "sk-desde-env" {
		t.Fatal("la API key debe leerse del entorno")
	}
	if _, err := Build(ProviderConfig{Name: "x", Type: "nope", BaseURL: "h"}); err == nil {
		t.Fatal("tipo desconocido debe fallar")
	}
}
