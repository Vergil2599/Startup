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

// OpenAICompat habla el protocolo /v1/chat/completions, que cubre LM Studio,
// OpenRouter, llama.cpp server, vLLM y cualquier "OpenAI compatible API".
type OpenAICompat struct {
	name    string
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAICompat crea el proveedor.
func NewOpenAICompat(name, baseURL, apiKey, model string, timeout time.Duration) *OpenAICompat {
	return &OpenAICompat{
		name: name, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey,
		model: model, client: &http.Client{Timeout: timeout},
	}
}

// Name devuelve el nombre configurado del proveedor.
func (o *OpenAICompat) Name() string { return o.name }

// Complete ejecuta una petición de chat no-streaming.
func (o *OpenAICompat) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = o.model
	}
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := map[string]any{
		"model":    model,
		"messages": []msg{},
	}
	msgs := []msg{}
	if req.System != "" {
		msgs = append(msgs, msg{"system", req.System})
	}
	msgs = append(msgs, msg{"user", req.Prompt})
	body["messages"] = msgs
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	payload, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Response{}, fmt.Errorf("proveedor %s: respuesta inválida: %w", o.name, err)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("proveedor %s: respuesta sin choices", o.name)
	}
	return Response{
		Text: out.Choices[0].Message.Content,
		TokensIn: out.Usage.PromptTokens, TokensOut: out.Usage.CompletionTokens,
		Latency: time.Since(start),
	}, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
