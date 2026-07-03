// Package mcpserver expone la biblioteca de PES como servidor MCP
// (Model Context Protocol) por stdio: JSON-RPC 2.0, un mensaje por línea.
// Con esto, Claude Code, Cursor o cualquier agente compatible con MCP puede
// buscar, leer, renderizar y validar los prompts del workspace del usuario.
package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Vergil2599/startup/pes/internal/core"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
	"github.com/Vergil2599/startup/pes/internal/template"
)

// ProtocolVersion es la versión del protocolo MCP que habla este servidor.
const ProtocolVersion = "2024-11-05"

// Server sirve el protocolo MCP sobre un par reader/writer.
type Server struct {
	studio  *core.Studio
	Version string
}

// New crea el servidor MCP sobre un Studio ya abierto.
func New(studio *core.Studio, version string) *Server {
	return &Server{studio: studio, Version: version}
}

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // ausente en notificaciones
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcOut struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve procesa mensajes hasta EOF. Los errores de un mensaje no tumban el bucle.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 32<<20)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // línea no-JSON: ignorar
		}
		if msg.ID == nil {
			continue // notificación (p. ej. notifications/initialized): sin respuesta
		}
		out := rpcOut{JSONRPC: "2.0", ID: msg.ID}
		result, err := s.dispatch(msg.Method, msg.Params)
		if err != nil {
			out.Error = &rpcErr{Code: -32603, Message: err.Error()}
			if strings.Contains(err.Error(), "método desconocido") {
				out.Error.Code = -32601
			}
		} else {
			out.Result = result
		}
		if err := enc.Encode(out); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) dispatch(method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "pes", "version": s.Version},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(params, &call); err != nil {
			return nil, fmt.Errorf("params inválidos: %w", err)
		}
		text, err := s.callTool(call.Name, call.Arguments)
		if err != nil {
			// Errores de herramienta se devuelven como resultado isError
			// (así el agente puede leerlos), no como fallo de protocolo.
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil
		}
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		}, nil
	default:
		return nil, fmt.Errorf("método desconocido: %s", method)
	}
}

// toolDefs declara las herramientas MCP con sus esquemas de entrada.
func toolDefs() []map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	str := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	return []map[string]any{
		{
			"name":        "search_prompts",
			"description": "Busca prompts en la biblioteca por texto libre (full-text, admite prefijos). Devuelve id, título, ruta, tags y puntuación de calidad.",
			"inputSchema": obj(map[string]any{"query": str("texto a buscar")}, "query"),
		},
		{
			"name":        "list_prompts",
			"description": "Lista los prompts de la biblioteca, opcionalmente filtrados por tag o categoría.",
			"inputSchema": obj(map[string]any{
				"tag":      str("filtrar por etiqueta"),
				"category": str("filtrar por categoría"),
			}),
		},
		{
			"name":        "get_prompt",
			"description": "Devuelve un prompt completo (metadatos y bloques) por id o ruta.",
			"inputSchema": obj(map[string]any{"ref": str("id ULID o ruta relativa .md")}, "ref"),
		},
		{
			"name":        "render_prompt",
			"description": "Renderiza un prompt con sus variables resueltas y devuelve el texto final listo para usar como prompt de un LLM. Acepta overrides de variables.",
			"inputSchema": obj(map[string]any{
				"ref": str("id ULID o ruta relativa .md"),
				"variables": map[string]any{
					"type":        "object",
					"description": "overrides de variables {nombre: valor} con máxima precedencia",
				},
			}, "ref"),
		},
		{
			"name":        "validate_prompt",
			"description": "Valida un prompt y devuelve su puntuación 0-100 y los hallazgos (secciones faltantes, ambigüedad, contradicciones, variables sin definir…).",
			"inputSchema": obj(map[string]any{"ref": str("id ULID o ruta relativa .md")}, "ref"),
		},
	}
}

func (s *Server) callTool(name string, args json.RawMessage) (string, error) {
	switch name {
	case "search_prompts":
		var a struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &a); err != nil || strings.TrimSpace(a.Query) == "" {
			return "", fmt.Errorf("se requiere query")
		}
		hits, err := s.studio.Search(a.Query, 20)
		if err != nil {
			return "", err
		}
		return marshalHits(hits)
	case "list_prompts":
		var a struct {
			Tag      string `json:"tag"`
			Category string `json:"category"`
		}
		_ = json.Unmarshal(args, &a)
		hits, err := s.studio.List(sqlindex.ListFilter{Tag: a.Tag, Category: a.Category})
		if err != nil {
			return "", err
		}
		return marshalHits(hits)
	case "get_prompt":
		ref, err := refArg(args)
		if err != nil {
			return "", err
		}
		e, err := s.studio.Get(ref)
		if err != nil {
			return "", err
		}
		data, _, err := s.studio.Export(string(e.Prompt.ID), "json", "")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "render_prompt":
		var a struct {
			Ref       string            `json:"ref"`
			Variables map[string]string `json:"variables"`
		}
		if err := json.Unmarshal(args, &a); err != nil || a.Ref == "" {
			return "", fmt.Errorf("se requiere ref")
		}
		e, err := s.studio.Get(a.Ref)
		if err != nil {
			return "", err
		}
		vars, err := s.studio.Vars("")
		if err != nil {
			return "", err
		}
		if len(a.Variables) > 0 {
			merged := map[string]string{}
			for k, v := range vars.Prompt {
				merged[k] = v
			}
			for k, v := range a.Variables {
				if !domain.ValidVariableName(k) {
					return "", fmt.Errorf("nombre de variable inválido: %q", k)
				}
				merged[k] = v
			}
			vars.Prompt = merged
		}
		return template.RenderPrompt(e.Prompt, vars, template.RenderOpts{})
	case "validate_prompt":
		ref, err := refArg(args)
		if err != nil {
			return "", err
		}
		report, err := s.studio.Validate(ref, "")
		if err != nil {
			return "", err
		}
		out, _ := json.MarshalIndent(map[string]any{
			"score":    report.Score,
			"findings": report.Findings,
		}, "", "  ")
		return string(out), nil
	default:
		return "", fmt.Errorf("herramienta desconocida: %s", name)
	}
}

func refArg(args json.RawMessage) (string, error) {
	var a struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(args, &a); err != nil || a.Ref == "" {
		return "", fmt.Errorf("se requiere ref")
	}
	return a.Ref, nil
}

func marshalHits(hits []sqlindex.Hit) (string, error) {
	type hit struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Path  string   `json:"path"`
		Tags  []string `json:"tags,omitempty"`
		Score *int     `json:"score,omitempty"`
	}
	out := make([]hit, 0, len(hits))
	for _, h := range hits {
		out = append(out, hit{ID: string(h.ID), Title: h.Title, Path: h.Path, Tags: h.Tags, Score: h.Score})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	return string(data), err
}
