package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ollama habla la API nativa de Ollama (/api/generate, sin streaming).
type Ollama struct {
	name    string
	baseURL string
	model   string
	client  *http.Client
}

// NewOllama crea el proveedor.
func NewOllama(name, baseURL, model string, timeout time.Duration) *Ollama {
	return &Ollama{
		name: name, baseURL: strings.TrimRight(baseURL, "/"),
		model: model, client: &http.Client{Timeout: timeout},
	}
}

// Name devuelve el nombre configurado del proveedor.
func (o *Ollama) Name() string { return o.name }

// Complete ejecuta una generación no-streaming.
func (o *Ollama) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = o.model
	}
	body := map[string]any{
		"model":  model,
		"prompt": req.Prompt,
		"stream": false,
	}
	if req.System != "" {
		body["system"] = req.System
	}
	opts := map[string]any{}
	if req.Temperature > 0 {
		opts["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		opts["num_predict"] = req.MaxTokens
	}
	if len(opts) > 0 {
		body["options"] = opts
	}
	payload, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("proveedor %s: %w", o.name, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("proveedor %s: HTTP %d: %s", o.name, resp.StatusCode, truncate(data, 300))
	}
	var out struct {
		Response        string `json:"response"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Response{}, fmt.Errorf("proveedor %s: respuesta inválida: %w", o.name, err)
	}
	return Response{
		Text: out.Response,
		TokensIn: out.PromptEvalCount, TokensOut: out.EvalCount,
		Latency: time.Since(start),
	}, nil
}
