// Package core contiene los casos de uso de PES. La UI (Wails) y la CLI
// consumen exactamente esta API: ninguna lógica de negocio vive fuera de aquí.
package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/export"
	"github.com/Vergil2599/startup/pes/internal/importer"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
	"github.com/Vergil2599/startup/pes/internal/template"
	"github.com/Vergil2599/startup/pes/internal/validation"
)

// Studio orquesta los casos de uso sobre un workspace abierto.
type Studio struct {
	WS        *fsrepo.Workspace
	Index     *sqlindex.Index
	Validator *validation.Engine
}

// NewStudio compone el servicio de aplicación.
func NewStudio(ws *fsrepo.Workspace, ix *sqlindex.Index) *Studio {
	return &Studio{WS: ws, Index: ix, Validator: validation.NewEngine()}
}

// CreateOpts controla la creación de un prompt.
type CreateOpts struct {
	Title      string
	Category   string
	TemplateID domain.ID // opcional: instancia una plantilla (con herencia)
	Tags       []string
}

// CreatePrompt crea un prompt nuevo (en blanco o desde plantilla) y lo persiste.
func (s *Studio) CreatePrompt(opts CreateOpts) (*domain.Prompt, string, error) {
	now := time.Now().UTC().Truncate(time.Second)
	p := &domain.Prompt{
		ID: domain.NewID(), Title: strings.TrimSpace(opts.Title),
		Category: opts.Category, Tags: domain.NormalizeTags(opts.Tags),
		CreatedAt: now, UpdatedAt: now, Schema: domain.SchemaVersion,
	}
	if opts.TemplateID != "" {
		tpl, err := s.ResolveTemplate(opts.TemplateID)
		if err != nil {
			return nil, "", fmt.Errorf("plantilla: %w", err)
		}
		p.Blocks = tpl.Blocks
		p.Variables = tpl.Variables
		p.TemplateID = opts.TemplateID
		if p.Category == "" {
			p.Category = tpl.Category
		}
	} else {
		// Esqueleto por defecto: las tres secciones esenciales.
		for _, t := range []domain.BlockType{domain.BlockRole, domain.BlockObjective, domain.BlockOutput} {
			p.Blocks = append(p.Blocks, domain.Block{Type: t, Enabled: true})
		}
	}
	rel, _, err := s.WS.SavePrompt(p, "")
	if err != nil {
		return nil, "", err
	}
	if _, err := s.Index.Sync(s.WS); err != nil {
		return nil, "", err
	}
	return p, rel, nil
}

// Save persiste cambios de un prompt existente y reindexa.
func (s *Studio) Save(p *domain.Prompt, relPath string) (string, error) {
	p.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	rel, _, err := s.WS.SavePrompt(p, relPath)
	if err != nil {
		return "", err
	}
	_, err = s.Index.Sync(s.WS)
	return rel, err
}

// Get carga un prompt por ID (vía índice, con fallback a escaneo) o por ruta.
func (s *Studio) Get(ref string) (*fsrepo.Entry, error) {
	if strings.HasSuffix(ref, ".md") {
		return s.WS.LoadPrompt(ref)
	}
	hits, err := s.Index.List(sqlindex.ListFilter{})
	if err == nil {
		for _, h := range hits {
			if string(h.ID) == ref {
				return s.WS.LoadPrompt(h.Path)
			}
		}
	}
	return s.WS.FindByID(domain.ID(ref))
}

// ResolveTemplate carga una plantilla por ID y materializa su herencia.
func (s *Studio) ResolveTemplate(id domain.ID) (*domain.Prompt, error) {
	e, err := s.WS.FindByID(id)
	if err != nil {
		return nil, err
	}
	return template.ResolveInheritance(e.Prompt, func(pid domain.ID) (*domain.Prompt, error) {
		pe, perr := s.WS.FindByID(pid)
		if perr != nil {
			return nil, perr
		}
		return pe.Prompt, nil
	})
}

// Vars carga el conjunto de variables (global + proyecto) del workspace.
func (s *Studio) Vars(project string) (domain.VariableSet, error) {
	global, err := s.WS.LoadVariables("global")
	if err != nil {
		return domain.VariableSet{}, err
	}
	vs := domain.VariableSet{Global: global}
	if project != "" {
		proj, err := s.WS.LoadVariables(project)
		if err != nil {
			return domain.VariableSet{}, err
		}
		vs.Project = proj
	}
	return vs, nil
}

// Render produce el texto final de un prompt con variables resueltas.
func (s *Studio) Render(ref, project string, strict bool) (string, error) {
	e, err := s.Get(ref)
	if err != nil {
		return "", err
	}
	vars, err := s.Vars(project)
	if err != nil {
		return "", err
	}
	return template.RenderPrompt(e.Prompt, vars, template.RenderOpts{Strict: strict})
}

// Validate ejecuta el validador y persiste la puntuación en el índice.
func (s *Studio) Validate(ref, project string) (domain.ValidationReport, error) {
	e, err := s.Get(ref)
	if err != nil {
		return domain.ValidationReport{}, err
	}
	vars, err := s.Vars(project)
	if err != nil {
		return domain.ValidationReport{}, err
	}
	report := s.Validator.Validate(validation.Input{Prompt: e.Prompt, Vars: vars})
	_ = s.Index.SetScore(e.Prompt.ID, report.Score) // best effort: el índice es derivado
	return report, nil
}

// Export serializa un prompt en el formato pedido.
func (s *Studio) Export(ref, format, project string) ([]byte, string, error) {
	e, err := s.Get(ref)
	if err != nil {
		return nil, "", err
	}
	codec, err := export.Get(format)
	if err != nil {
		return nil, "", err
	}
	vars, err := s.Vars(project)
	if err != nil {
		return nil, "", err
	}
	data, err := codec.Export(export.Input{Prompt: e.Prompt, Vars: vars})
	return data, codec.Extension(), err
}

// Import convierte datos externos en un prompt del workspace.
func (s *Studio) Import(data []byte, title string) (*domain.Prompt, string, error) {
	p, err := importer.Import(data, title)
	if err != nil {
		return nil, "", err
	}
	if p.CreatedAt.IsZero() {
		now := time.Now().UTC().Truncate(time.Second)
		p.CreatedAt, p.UpdatedAt = now, now
	}
	rel, _, err := s.WS.SavePrompt(p, "")
	if err != nil {
		return nil, "", err
	}
	if _, err := s.Index.Sync(s.WS); err != nil {
		return nil, "", err
	}
	return p, rel, nil
}

// Search busca en el índice full-text.
func (s *Studio) Search(query string, limit int) ([]sqlindex.Hit, error) {
	return s.Index.Search(query, limit)
}

// List lista la biblioteca con filtros.
func (s *Studio) List(f sqlindex.ListFilter) ([]sqlindex.Hit, error) {
	return s.Index.List(f)
}

// Reindex sincroniza (o reconstruye) el índice desde el workspace.
func (s *Studio) Reindex(rebuild bool) (sqlindex.SyncStats, error) {
	if rebuild {
		return s.Index.Rebuild(s.WS)
	}
	return s.Index.Sync(s.WS)
}
