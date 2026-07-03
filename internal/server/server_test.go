package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/core"
	"github.com/Vergil2599/startup/pes/internal/optimize"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
)

func newTestServer(t *testing.T) (*httptest.Server, *core.Studio) {
	t.Helper()
	ws, err := fsrepo.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ix, err := sqlindex.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ix.Close() })
	studio := core.NewStudio(ws, ix)
	srv := httptest.NewServer(New(studio, nil, "test").Handler())
	t.Cleanup(srv.Close)
	return srv, studio
}

func call(t *testing.T, srv *httptest.Server, method, path string, body any, out any) *http.Response {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, srv.URL+path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return resp
}

func TestFullAPILifecycle(t *testing.T) {
	srv, _ := newTestServer(t)

	// health
	var h map[string]any
	call(t, srv, "GET", "/api/health", nil, &h)
	if h["ai_available"] != false {
		t.Fatal("sin sidecar, ai_available debe ser false")
	}

	// crear
	var p promptDTO
	resp := call(t, srv, "POST", "/api/prompts", map[string]any{
		"title": "Desde API", "tags": []string{"api", "Test"},
	}, &p)
	if resp.StatusCode != 200 || p.ID == "" || len(p.Blocks) != 3 {
		t.Fatalf("create: %d %+v", resp.StatusCode, p)
	}

	// actualizar bloques
	p.Blocks[1].Content = "Objetivo: detectar 3 defectos con {{tool}}."
	p.Description = "desc"
	var updated promptDTO
	call(t, srv, "PUT", "/api/prompts/"+p.ID, p, &updated)
	if updated.Blocks[1].Content != p.Blocks[1].Content || updated.Description != "desc" {
		t.Fatalf("update: %+v", updated)
	}

	// validar: variable sin definir aparece
	var report struct {
		Score    int
		Findings []struct{ RuleID string }
	}
	call(t, srv, "POST", "/api/prompts/"+p.ID+"/validate", map[string]any{}, &report)
	found := false
	for _, f := range report.Findings {
		if f.RuleID == "undefined_variables" {
			found = true
		}
	}
	if !found {
		t.Fatalf("la variable sin definir debe detectarse: %+v", report)
	}

	// definir variable global → render la resuelve
	call(t, srv, "PUT", "/api/variables", map[string]string{"tool": "golangci-lint"}, nil)
	var rendered map[string]any
	call(t, srv, "POST", "/api/prompts/"+p.ID+"/render", map[string]any{}, &rendered)
	if !strings.Contains(rendered["text"].(string), "golangci-lint") {
		t.Fatalf("render: %v", rendered)
	}

	// snapshot + historial + restore
	call(t, srv, "POST", "/api/prompts/"+p.ID+"/snapshot", map[string]string{"comment": "v1"}, nil)
	p.Blocks[1].Content = "cambiado"
	call(t, srv, "PUT", "/api/prompts/"+p.ID, p, nil)
	call(t, srv, "POST", "/api/prompts/"+p.ID+"/snapshot", map[string]string{"comment": "v2"}, nil)
	var versions []map[string]any
	call(t, srv, "GET", "/api/prompts/"+p.ID+"/history", nil, &versions)
	if len(versions) != 2 {
		t.Fatalf("historial: %d", len(versions))
	}
	call(t, srv, "POST", "/api/prompts/"+p.ID+"/restore", map[string]string{"version": "1"}, nil)
	var after promptDTO
	call(t, srv, "GET", "/api/prompts/"+p.ID, nil, &after)
	if !strings.Contains(after.Blocks[1].Content, "detectar 3 defectos") {
		t.Fatalf("restore no aplicado: %q", after.Blocks[1].Content)
	}

	// export
	var exp map[string]string
	call(t, srv, "POST", "/api/prompts/"+p.ID+"/export", map[string]string{"format": "html"}, &exp)
	if exp["extension"] != ".html" || exp["data_base64"] == "" {
		t.Fatalf("export: %v", exp)
	}

	// búsqueda y listado
	var hits []map[string]any
	call(t, srv, "GET", "/api/search?q=defectos", nil, &hits)
	if len(hits) != 1 {
		t.Fatalf("search: %v", hits)
	}
	call(t, srv, "GET", "/api/prompts?tag=api", nil, &hits)
	if len(hits) != 1 {
		t.Fatalf("list por tag: %v", hits)
	}

	// optimize sin sidecar → 503
	resp = call(t, srv, "POST", "/api/prompts/"+p.ID+"/optimize", map[string]any{}, nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("optimize sin IA: %d", resp.StatusCode)
	}

	// delete
	call(t, srv, "DELETE", "/api/prompts/"+p.ID, nil, nil)
	resp = call(t, srv, "GET", "/api/prompts/"+p.ID, nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("tras delete: %d", resp.StatusCode)
	}
}

func TestComposeAndDiffEndpoints(t *testing.T) {
	srv, _ := newTestServer(t)
	var a, b promptDTO
	call(t, srv, "POST", "/api/prompts", map[string]any{"title": "Base"}, &a)
	call(t, srv, "POST", "/api/prompts", map[string]any{"title": "Overlay"}, &b)
	a.Blocks[0].Content = "rol base"
	b.Blocks[0].Content = "rol overlay"
	call(t, srv, "PUT", "/api/prompts/"+a.ID, a, nil)
	call(t, srv, "PUT", "/api/prompts/"+b.ID, b, nil)

	var composed promptDTO
	call(t, srv, "POST", "/api/compose", map[string]any{
		"title": "Final", "strategy": "concat",
		"layers": []map[string]any{{"ref": a.ID, "enabled": true}, {"ref": b.ID, "enabled": true}},
	}, &composed)
	if composed.ID == "" {
		t.Fatalf("compose: %+v", composed)
	}
	var full promptDTO
	call(t, srv, "GET", "/api/prompts/"+composed.ID, nil, &full)
	if !strings.Contains(full.Blocks[0].Content, "rol base") || !strings.Contains(full.Blocks[0].Content, "rol overlay") {
		t.Fatalf("concat: %+v", full.Blocks)
	}

	var d struct {
		Structural struct{ Blocks []struct{ Type, Kind string } }
		TextOps    []struct{ Kind, Line string } `json:"text_ops"`
	}
	call(t, srv, "GET", "/api/diff?a="+a.ID+"&b="+b.ID, nil, &d)
	if len(d.TextOps) == 0 {
		t.Fatalf("diff: %+v", d)
	}
}

func TestRunEndpointWithMockProvider(t *testing.T) {
	srv, studio := newTestServer(t)
	// Mock OpenAI-compatible.
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "respuesta simulada"}}},
			"usage":   map[string]int{"prompt_tokens": 3, "completion_tokens": 2},
		})
	}))
	defer llm.Close()
	cfg := fmt.Sprintf("providers:\n  - {name: mock, type: openai, base_url: %q, model: m1}\n", llm.URL)
	os.WriteFile(filepath.Join(studio.WS.Root, ".pes", "providers.yaml"), []byte(cfg), 0o644)

	var p promptDTO
	call(t, srv, "POST", "/api/prompts", map[string]any{"title": "Ejecutable"}, &p)

	var provs []map[string]string
	call(t, srv, "GET", "/api/providers", nil, &provs)
	if len(provs) != 1 || provs[0]["name"] != "mock" {
		t.Fatalf("providers: %v", provs)
	}

	var results []map[string]any
	call(t, srv, "POST", "/api/run", map[string]any{"ref": p.ID, "providers": []string{"mock"}}, &results)
	if len(results) != 1 || results[0]["response"] != "respuesta simulada" {
		t.Fatalf("run: %v", results)
	}

	var runs []map[string]any
	call(t, srv, "GET", "/api/prompts/"+p.ID+"/runs", nil, &runs)
	if len(runs) != 1 {
		t.Fatalf("runs persistidos: %v", runs)
	}

	// Proveedor desconocido → error controlado.
	resp := call(t, srv, "POST", "/api/run", map[string]any{"ref": p.ID, "providers": []string{"nope"}}, nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("proveedor desconocido: %d", resp.StatusCode)
	}
}

func TestOptimizeEndpointWithRealSidecar(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 no disponible")
	}
	main, _ := filepath.Abs("../../sidecar/pes_ai/main.py")
	client, err := optimize.New([]string{py, main})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })

	ws, _ := fsrepo.Init(t.TempDir())
	ix, _ := sqlindex.Open(":memory:")
	t.Cleanup(func() { ix.Close() })
	studio := core.NewStudio(ws, ix)
	srv := httptest.NewServer(New(studio, client, "test").Handler())
	t.Cleanup(srv.Close)

	var p promptDTO
	call(t, srv, "POST", "/api/prompts", map[string]any{"title": "Optimizable"}, &p)
	p.Blocks[1].Content = "Por favor, ayudar al usuario."
	call(t, srv, "PUT", "/api/prompts/"+p.ID, p, nil)

	var sugs []map[string]any
	resp := call(t, srv, "POST", "/api/prompts/"+p.ID+"/optimize", map[string]any{}, &sugs)
	if resp.StatusCode != 200 || len(sugs) == 0 {
		t.Fatalf("optimize: %d %v", resp.StatusCode, sugs)
	}
}

func TestUIServed(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if !strings.Contains(buf.String(), "Prompt Engineering Studio") {
		t.Fatal("la UI embebida no se sirve")
	}
	if resp2, _ := http.Get(srv.URL + "/otra-ruta"); resp2.StatusCode != http.StatusNotFound {
		t.Fatal("rutas desconocidas deben dar 404")
	}
}

func TestBadJSONIs400(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Post(srv.URL+"/api/prompts", "application/json", strings.NewReader("{roto"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("JSON roto: %d", resp.StatusCode)
	}
}
