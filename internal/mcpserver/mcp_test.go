package mcpserver

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/core"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
)

func newMCP(t *testing.T) (*Server, *core.Studio) {
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
	return New(studio, "test"), studio
}

// exchange envía mensajes JSON-RPC por líneas y devuelve las respuestas.
func exchange(t *testing.T, s *Server, msgs ...string) []map[string]any {
	t.Helper()
	var in bytes.Buffer
	for _, m := range msgs {
		in.WriteString(m + "\n")
	}
	var out bytes.Buffer
	if err := s.Serve(&in, &out); err != nil {
		t.Fatal(err)
	}
	var resps []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("respuesta no JSON: %q", line)
		}
		resps = append(resps, r)
	}
	return resps
}

func result(t *testing.T, r map[string]any) map[string]any {
	t.Helper()
	if e, ok := r["error"]; ok && e != nil {
		t.Fatalf("error inesperado: %v", e)
	}
	return r["result"].(map[string]any)
}

// toolText extrae el texto del resultado de tools/call.
func toolText(t *testing.T, r map[string]any) (string, bool) {
	t.Helper()
	res := result(t, r)
	content := res["content"].([]any)[0].(map[string]any)
	isErr, _ := res["isError"].(bool)
	return content["text"].(string), isErr
}

func TestInitializeHandshake(t *testing.T) {
	s, _ := newMCP(t)
	resps := exchange(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // notificación: sin respuesta
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	)
	if len(resps) != 2 {
		t.Fatalf("esperaba 2 respuestas (la notificación no responde): %d", len(resps))
	}
	res := result(t, resps[0])
	if res["protocolVersion"] != ProtocolVersion {
		t.Fatalf("initialize: %v", res)
	}
	info := res["serverInfo"].(map[string]any)
	if info["name"] != "pes" {
		t.Fatalf("serverInfo: %v", info)
	}
}

func TestToolsListDeclaresFive(t *testing.T) {
	s, _ := newMCP(t)
	resps := exchange(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tools := result(t, resps[0])["tools"].([]any)
	if len(tools) != 5 {
		t.Fatalf("tools: %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		tool := tl.(map[string]any)
		names[tool["name"].(string)] = true
		if tool["inputSchema"] == nil || tool["description"] == "" {
			t.Fatalf("tool sin schema/descripción: %v", tool)
		}
	}
	for _, want := range []string{"search_prompts", "list_prompts", "get_prompt", "render_prompt", "validate_prompt"} {
		if !names[want] {
			t.Fatalf("falta tool %s", want)
		}
	}
}

func seedPrompt(t *testing.T, studio *core.Studio) *domain.Prompt {
	t.Helper()
	p, rel, err := studio.CreatePrompt(core.CreateOpts{Title: "Revisor Go", Tags: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true,
		Content: "Detectar 3 defectos usando {{tool}}."})
	if _, err := studio.Save(p, rel); err != nil {
		t.Fatal(err)
	}
	studio.WS.SaveVariables("global", map[string]string{"tool": "vet"})
	return p
}

func TestToolFlow(t *testing.T) {
	s, studio := newMCP(t)
	p := seedPrompt(t, studio)

	// search
	resps := exchange(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_prompts","arguments":{"query":"defectos"}}}`)
	text, isErr := toolText(t, resps[0])
	if isErr || !strings.Contains(text, "Revisor Go") {
		t.Fatalf("search: %s", text)
	}

	// get
	resps = exchange(t, s,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_prompt","arguments":{"ref":"`+string(p.ID)+`"}}}`)
	text, _ = toolText(t, resps[0])
	if !strings.Contains(text, `"title": "Revisor Go"`) {
		t.Fatalf("get: %s", text)
	}

	// render con variable global
	resps = exchange(t, s,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"render_prompt","arguments":{"ref":"`+string(p.ID)+`"}}}`)
	text, _ = toolText(t, resps[0])
	if !strings.Contains(text, "usando vet.") {
		t.Fatalf("render: %s", text)
	}

	// render con override de variables (máxima precedencia)
	resps = exchange(t, s,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"render_prompt","arguments":{"ref":"`+string(p.ID)+`","variables":{"tool":"golangci-lint"}}}}`)
	text, _ = toolText(t, resps[0])
	if !strings.Contains(text, "golangci-lint") {
		t.Fatalf("override: %s", text)
	}

	// validate
	resps = exchange(t, s,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"validate_prompt","arguments":{"ref":"`+string(p.ID)+`"}}}`)
	text, _ = toolText(t, resps[0])
	if !strings.Contains(text, `"score"`) {
		t.Fatalf("validate: %s", text)
	}
}

func TestToolErrorsAreIsErrorNotProtocolFailures(t *testing.T) {
	s, _ := newMCP(t)
	resps := exchange(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_prompt","arguments":{"ref":"NOEXISTE"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tool_falsa","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"metodo/inexistente"}`,
	)
	if _, isErr := toolText(t, resps[0]); !isErr {
		t.Fatal("ref inexistente debe ser isError")
	}
	if _, isErr := toolText(t, resps[1]); !isErr {
		t.Fatal("tool desconocida debe ser isError")
	}
	if e := resps[2]["error"].(map[string]any); e["code"].(float64) != -32601 {
		t.Fatalf("método desconocido debe ser -32601: %v", e)
	}
}

func TestServeSurvivesGarbageLines(t *testing.T) {
	s, _ := newMCP(t)
	resps := exchange(t, s,
		"esto no es json",
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
	)
	if len(resps) != 1 {
		t.Fatalf("el bucle debe sobrevivir a basura: %d respuestas", len(resps))
	}
}
