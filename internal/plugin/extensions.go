package plugin

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/validation"
)

// PromptIR es la representación intermedia estable que ven los plugins.
// Nunca se les pasan structs internos del host.
type PromptIR struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Category    string            `json:"category,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Blocks      []BlockIR         `json:"blocks"`
}

// BlockIR es un bloque en la representación intermedia.
type BlockIR struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Content string `json:"content"`
}

// ToIR convierte un prompt del dominio a su IR.
func ToIR(p *domain.Prompt) PromptIR {
	ir := PromptIR{
		ID: string(p.ID), Title: p.Title, Description: p.Description,
		Tags: p.Tags, Category: p.Category, Variables: p.Variables,
	}
	for _, b := range p.Blocks {
		ir.Blocks = append(ir.Blocks, BlockIR{Type: string(b.Type), Enabled: b.Enabled, Content: b.Content})
	}
	return ir
}

// ── llm.provider ─────────────────────────────────────────────────────────────

// ProviderAdapter expone un plugin como ai.Provider del core.
type ProviderAdapter struct {
	proc *Proc
	name string
}

// NewProviderAdapter crea el adaptador.
func NewProviderAdapter(proc *Proc) *ProviderAdapter {
	return &ProviderAdapter{proc: proc, name: "plugin:" + proc.Manifest.Plugin.ID}
}

// Name implementa ai.Provider.
func (a *ProviderAdapter) Name() string { return a.name }

// Complete implementa ai.Provider delegando en el plugin.
func (a *ProviderAdapter) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	params := map[string]any{
		"model": req.Model, "prompt": req.Prompt, "system": req.System,
		"temperature": req.Temperature, "max_tokens": req.MaxTokens,
	}
	var out struct {
		Text      string `json:"text"`
		TokensIn  int    `json:"tokens_in"`
		TokensOut int    `json:"tokens_out"`
	}
	start := time.Now()
	if err := a.proc.Call(ctx, "llm.complete", params, &out); err != nil {
		return ai.Response{}, err
	}
	return ai.Response{Text: out.Text, TokensIn: out.TokensIn, TokensOut: out.TokensOut,
		Latency: time.Since(start)}, nil
}

// ── validation.rule ──────────────────────────────────────────────────────────

// RuleAdapter expone un plugin como regla del Validation Engine.
type RuleAdapter struct {
	proc *Proc
}

// NewRuleAdapter crea el adaptador.
func NewRuleAdapter(proc *Proc) *RuleAdapter { return &RuleAdapter{proc: proc} }

// ID implementa validation.Rule.
func (r *RuleAdapter) ID() string { return "plugin:" + r.proc.Manifest.Plugin.ID }

// Check implementa validation.Rule. Un plugin caído no rompe la validación:
// devuelve cero hallazgos (el gestor de plugins muestra el estado suspendido).
func (r *RuleAdapter) Check(in validation.Input) []domain.Finding {
	var out struct {
		Findings []struct {
			Severity   string `json:"severity"`
			BlockType  string `json:"block_type"`
			Message    string `json:"message"`
			Suggestion string `json:"suggestion"`
		} `json:"findings"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.proc.Call(ctx, "validation.check", map[string]any{"prompt": ToIR(in.Prompt)}, &out); err != nil {
		return nil
	}
	var findings []domain.Finding
	for _, f := range out.Findings {
		sev := domain.Severity(f.Severity)
		if sev != domain.SeverityError && sev != domain.SeverityWarn && sev != domain.SeverityInfo {
			sev = domain.SeverityInfo
		}
		findings = append(findings, domain.Finding{
			Severity: sev, BlockType: domain.BlockType(f.BlockType),
			Message: f.Message, Suggestion: f.Suggestion,
		})
	}
	return findings
}

// ── export.codec ─────────────────────────────────────────────────────────────

// ExportResult es la salida de un codec de plugin.
type ExportResult struct {
	Data      []byte
	Extension string
}

// ExportWith pide al plugin exportar un prompt en su formato.
func ExportWith(ctx context.Context, proc *Proc, p *domain.Prompt, format string) (ExportResult, error) {
	var out struct {
		DataBase64 string `json:"data_base64"`
		Extension  string `json:"extension"`
	}
	if err := proc.Call(ctx, "export.export",
		map[string]any{"format": format, "prompt": ToIR(p)}, &out); err != nil {
		return ExportResult{}, err
	}
	data, err := base64.StdEncoding.DecodeString(out.DataBase64)
	if err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Data: data, Extension: out.Extension}, nil
}
