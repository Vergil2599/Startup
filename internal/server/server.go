// Package server expone los casos de uso de PES como API JSON en localhost y
// sirve la UI web embebida. Es la misma capa de vista fina que envolverá el
// shell de escritorio (Wails): aquí no vive ninguna lógica de negocio.
package server

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/composer"
	"github.com/Vergil2599/startup/pes/internal/core"
	"github.com/Vergil2599/startup/pes/internal/diff"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/history"
	"github.com/Vergil2599/startup/pes/internal/optimize"
	"github.com/Vergil2599/startup/pes/internal/plugin"
	"github.com/Vergil2599/startup/pes/internal/run"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
)

//go:embed ui/index.html
var uiFS embed.FS

// Server enruta la API local de PES.
type Server struct {
	studio  *core.Studio
	hist    *history.Store
	runner  *run.Service
	optim   *optimize.Client // nil si el sidecar no está disponible
	Version string
}

// New construye el servidor. optim puede ser nil (IA no disponible).
func New(studio *core.Studio, optim *optimize.Client, version string) *Server {
	return &Server{
		studio: studio, hist: history.New(studio.WS),
		runner: run.NewService(studio.WS), optim: optim, Version: version,
	}
}

// Handler devuelve el mux completo (API + UI estática).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/prompts", s.listPrompts)
	mux.HandleFunc("POST /api/prompts", s.createPrompt)
	mux.HandleFunc("GET /api/prompts/{ref}", s.getPrompt)
	mux.HandleFunc("PUT /api/prompts/{ref}", s.updatePrompt)
	mux.HandleFunc("DELETE /api/prompts/{ref}", s.deletePrompt)
	mux.HandleFunc("POST /api/prompts/{ref}/render", s.renderPrompt)
	mux.HandleFunc("POST /api/prompts/{ref}/validate", s.validatePrompt)
	mux.HandleFunc("POST /api/prompts/{ref}/export", s.exportPrompt)
	mux.HandleFunc("POST /api/prompts/{ref}/snapshot", s.snapshot)
	mux.HandleFunc("GET /api/prompts/{ref}/history", s.listHistory)
	mux.HandleFunc("POST /api/prompts/{ref}/restore", s.restore)
	mux.HandleFunc("GET /api/prompts/{ref}/runs", s.listRuns)
	mux.HandleFunc("POST /api/prompts/{ref}/optimize", s.optimizePrompt)
	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("GET /api/diff", s.diffPrompts)
	mux.HandleFunc("POST /api/compose", s.compose)
	mux.HandleFunc("GET /api/templates", s.listTemplates)
	mux.HandleFunc("GET /api/variables", s.getVariables)
	mux.HandleFunc("PUT /api/variables", s.putVariables)
	mux.HandleFunc("GET /api/providers", s.listProviders)
	mux.HandleFunc("POST /api/run", s.runPrompt)
	mux.HandleFunc("GET /api/plugins", s.listPlugins)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := uiFS.ReadFile("ui/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	return mux
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

type promptDTO struct {
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category"`
	Favorite    bool              `json:"favorite"`
	Variables   map[string]string `json:"variables"`
	Blocks      []blockDTO        `json:"blocks"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type blockDTO struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Content string `json:"content"`
}

func toDTO(e *fsrepo.Entry) promptDTO {
	p := e.Prompt
	dto := promptDTO{
		ID: string(p.ID), Path: e.Path, Title: p.Title, Description: p.Description,
		Tags: p.Tags, Category: p.Category, Favorite: p.Favorite,
		Variables: p.Variables, UpdatedAt: p.UpdatedAt,
	}
	if dto.Tags == nil {
		dto.Tags = []string{}
	}
	if dto.Variables == nil {
		dto.Variables = map[string]string{}
	}
	for _, b := range p.Blocks {
		dto.Blocks = append(dto.Blocks, blockDTO{Type: string(b.Type), Enabled: b.Enabled, Content: b.Content})
	}
	return dto
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"version": s.Version, "ai_available": s.optim != nil,
	})
}

func (s *Server) listPrompts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hits, err := s.studio.List(sqlindex.ListFilter{
		Tag: q.Get("tag"), Category: q.Get("category"), Favorites: q.Get("favorites") == "true",
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	if hits == nil {
		hits = []sqlindex.Hit{}
	}
	writeJSON(w, hits)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	hits, err := s.studio.Search(r.URL.Query().Get("q"), 50)
	if err != nil {
		writeErr(w, err)
		return
	}
	if hits == nil {
		hits = []sqlindex.Hit{}
	}
	writeJSON(w, hits)
}

func (s *Server) createPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title      string   `json:"title"`
		Category   string   `json:"category"`
		Tags       []string `json:"tags"`
		TemplateID string   `json:"template_id"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	p, rel, err := s.studio.CreatePrompt(core.CreateOpts{
		Title: req.Title, Category: req.Category, Tags: req.Tags,
		TemplateID: domain.ID(req.TemplateID),
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, toDTO(&fsrepo.Entry{Prompt: p, Path: rel}))
}

func (s *Server) getPrompt(w http.ResponseWriter, r *http.Request) {
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, toDTO(e))
}

func (s *Server) updatePrompt(w http.ResponseWriter, r *http.Request) {
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	var req promptDTO
	if !readJSON(w, r, &req) {
		return
	}
	p := e.Prompt
	p.Title = req.Title
	p.Description = req.Description
	p.Tags = domain.NormalizeTags(req.Tags)
	p.Category = req.Category
	p.Favorite = req.Favorite
	p.Variables = req.Variables
	p.Blocks = nil
	for _, b := range req.Blocks {
		p.Blocks = append(p.Blocks, domain.Block{Type: domain.BlockType(b.Type), Enabled: b.Enabled, Content: b.Content})
	}
	if err := p.Validate(); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := s.studio.Save(p, e.Path); err != nil {
		writeErr(w, err)
		return
	}
	e2, err := s.studio.Get(e.Path)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, toDTO(e2))
}

func (s *Server) deletePrompt(w http.ResponseWriter, r *http.Request) {
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.studio.WS.DeletePrompt(e.Path); err != nil {
		writeErr(w, err)
		return
	}
	s.studio.Reindex(false)
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) renderPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
		Strict  bool   `json:"strict"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	out, err := s.studio.Render(r.PathValue("ref"), req.Project, req.Strict)
	if err != nil && !errors.Is(err, domain.ErrUnresolvedVars) {
		writeErr(w, err)
		return
	}
	resp := map[string]any{"text": out}
	if err != nil {
		resp["warning"] = err.Error()
	}
	writeJSON(w, resp)
}

func (s *Server) validatePrompt(w http.ResponseWriter, r *http.Request) {
	report, err := s.studio.Validate(r.PathValue("ref"), "")
	if err != nil {
		writeErr(w, err)
		return
	}
	if report.Findings == nil {
		report.Findings = []domain.Finding{}
	}
	writeJSON(w, report)
}

func (s *Server) exportPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Format string `json:"format"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	data, ext, err := s.studio.Export(r.PathValue("ref"), req.Format, "")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{
		"data_base64": base64.StdEncoding.EncodeToString(data),
		"extension":   ext,
	})
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Comment string `json:"comment"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	v, err := s.hist.Snapshot(e.Path, "", req.Comment)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) listHistory(w http.ResponseWriter, r *http.Request) {
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	versions, err := s.hist.List(e.Prompt.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if versions == nil {
		versions = []domain.Version{}
	}
	writeJSON(w, versions)
}

func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	v, err := s.hist.Restore(e.Path, req.Version)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.studio.Reindex(false)
	writeJSON(w, v)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	results, err := s.runner.History(e.Prompt.ID, 50)
	if err != nil {
		writeErr(w, err)
		return
	}
	if results == nil {
		results = []run.Result{}
	}
	writeJSON(w, results)
}

func (s *Server) optimizePrompt(w http.ResponseWriter, r *http.Request) {
	if s.optim == nil {
		http.Error(w, `{"error":"pes-ai no está instalado (la optimización es opcional)"}`, http.StatusServiceUnavailable)
		return
	}
	e, err := s.studio.Get(r.PathValue("ref"))
	if err != nil {
		writeErr(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	sugs, err := s.optim.Suggest(ctx, e.Prompt)
	if err != nil {
		writeErr(w, err)
		return
	}
	if sugs == nil {
		sugs = []optimize.Suggestion{}
	}
	writeJSON(w, sugs)
}

func (s *Server) diffPrompts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ea, err := s.studio.Get(q.Get("a"))
	if err != nil {
		writeErr(w, err)
		return
	}
	eb, err := s.studio.Get(q.Get("b"))
	if err != nil {
		writeErr(w, err)
		return
	}
	structural := diff.Structural(ea.Prompt, eb.Prompt)
	ta, _ := s.studio.Render(q.Get("a"), "", false)
	tb, _ := s.studio.Render(q.Get("b"), "", false)
	if structural.Blocks == nil {
		structural.Blocks = []domain.BlockDiff{}
	}
	writeJSON(w, map[string]any{
		"structural": structural,
		"text_ops":   diff.Lines(ta, tb),
	})
}

func (s *Server) compose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Strategy string `json:"strategy"`
		Layers   []struct {
			Ref     string `json:"ref"`
			Enabled bool   `json:"enabled"`
		} `json:"layers"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	c := &composer.Composition{Title: req.Title, Strategy: composer.MergeStrategy(req.Strategy)}
	for _, l := range req.Layers {
		c.Layers = append(c.Layers, composer.Layer{Ref: l.Ref, Enabled: l.Enabled})
	}
	if c.Strategy == "" {
		c.Strategy = composer.StrategyConcat
	}
	p, err := composer.Compose(c, func(ref string) (*domain.Prompt, error) {
		e, gerr := s.studio.Get(ref)
		if gerr != nil {
			return nil, gerr
		}
		return e.Prompt, nil
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	rel, err := s.studio.Save(p, "")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, toDTO(&fsrepo.Entry{Prompt: p, Path: rel}))
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	type tplDTO struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Category string `json:"category"`
		Extends  string `json:"extends,omitempty"`
	}
	out := []tplDTO{}
	err := s.studio.WS.Walk(fsrepo.DirTemplates, func(e *fsrepo.Entry) {
		out = append(out, tplDTO{
			ID: string(e.Prompt.ID), Title: e.Prompt.Title,
			Category: e.Prompt.Category, Extends: string(e.Prompt.Extends),
		})
	}, nil)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) getVariables(w http.ResponseWriter, r *http.Request) {
	vars, err := s.studio.WS.LoadVariables("global")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, vars)
}

func (s *Server) putVariables(w http.ResponseWriter, r *http.Request) {
	var vars map[string]string
	if !readJSON(w, r, &vars) {
		return
	}
	if err := s.studio.WS.SaveVariables("global", vars); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, vars)
}

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	cfgs, err := ai.LoadConfigs(s.studio.WS.Root)
	if err != nil {
		writeErr(w, err)
		return
	}
	type provDTO struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Model string `json:"model"`
	}
	out := []provDTO{}
	for _, c := range cfgs {
		out = append(out, provDTO{Name: c.Name, Type: c.Type, Model: c.Model})
	}
	writeJSON(w, out)
}

func (s *Server) runPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Ref         string   `json:"ref"`
		Providers   []string `json:"providers"`
		Temperature float64  `json:"temperature"`
		MaxTokens   int      `json:"max_tokens"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	e, err := s.studio.Get(req.Ref)
	if err != nil {
		writeErr(w, err)
		return
	}
	rendered, err := s.studio.Render(req.Ref, "", false)
	if err != nil {
		writeErr(w, err)
		return
	}
	cfgs, err := ai.LoadConfigs(s.studio.WS.Root)
	if err != nil {
		writeErr(w, err)
		return
	}
	byName := map[string]ai.ProviderConfig{}
	for _, c := range cfgs {
		byName[c.Name] = c
	}
	var targets []run.Target
	for _, name := range req.Providers {
		cfg, ok := byName[name]
		if !ok {
			writeErr(w, fmt.Errorf("proveedor no configurado: %q", name))
			return
		}
		p, berr := ai.Build(cfg)
		if berr != nil {
			writeErr(w, berr)
			return
		}
		targets = append(targets, run.Target{Provider: p, Model: cfg.Model})
	}
	if len(targets) == 0 {
		writeErr(w, fmt.Errorf("no se indicó ningún proveedor"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	results, err := s.runner.Run(ctx, e.Prompt.ID, rendered, targets, run.Params{
		Temperature: req.Temperature, MaxTokens: req.MaxTokens,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, results)
}

func (s *Server) listPlugins(w http.ResponseWriter, r *http.Request) {
	ds, err := plugin.Discover(s.studio.WS.Root)
	if err != nil {
		writeErr(w, err)
		return
	}
	type plugDTO struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Version string   `json:"version"`
		Enabled bool     `json:"enabled"`
		Error   string   `json:"error,omitempty"`
		Exts    []string `json:"extension_points,omitempty"`
	}
	out := []plugDTO{}
	for _, d := range ds {
		dto := plugDTO{Enabled: d.Enabled, Error: d.LoadErr}
		if d.Manifest != nil {
			dto.ID = d.Manifest.Plugin.ID
			dto.Name = d.Manifest.Plugin.Name
			dto.Version = d.Manifest.Plugin.Version
			dto.Exts = d.Manifest.Contributes.ExtensionPoints
		}
		out = append(out, dto)
	}
	writeJSON(w, out)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, domain.ErrNotFound) {
		code = http.StatusNotFound
	} else if errors.Is(err, domain.ErrEmptyTitle) || errors.Is(err, domain.ErrDuplicateBlock) ||
		errors.Is(err, domain.ErrInvalidBlockType) || errors.Is(err, domain.ErrInvalidVarName) {
		code = http.StatusBadRequest
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "JSON inválido: " + err.Error()})
		return false
	}
	return true
}
