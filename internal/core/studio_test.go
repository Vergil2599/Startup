package core

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
)

func newStudio(t *testing.T) *Studio {
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
	return NewStudio(ws, ix)
}

func TestCreateGetRoundTrip(t *testing.T) {
	s := newStudio(t)
	p, rel, err := s.CreatePrompt(CreateOpts{Title: "Mi Prompt", Category: "writing", Tags: []string{"Test"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Blocks) != 3 {
		t.Fatalf("esqueleto por defecto: %d bloques", len(p.Blocks))
	}
	byID, err := s.Get(string(p.ID))
	if err != nil || byID.Prompt.Title != "Mi Prompt" {
		t.Fatalf("Get por ID: %v", err)
	}
	byPath, err := s.Get(rel)
	if err != nil || byPath.Prompt.ID != p.ID {
		t.Fatalf("Get por ruta: %v", err)
	}
	if _, err := s.Get("01INEXISTENTE0000000000000"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("esperaba ErrNotFound: %v", err)
	}
}

func TestCreateFromTemplateWithInheritance(t *testing.T) {
	s := newStudio(t)
	base := &domain.Prompt{
		ID: domain.NewID(), Title: "Base coding", Category: "software-development",
		Variables: map[string]string{"style": "clean"},
		Blocks: []domain.Block{
			{Type: domain.BlockRole, Enabled: true, Content: "Eres un ingeniero {{style}}."},
			{Type: domain.BlockRules, Enabled: true, Content: "Sigue SOLID."},
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Schema: 1,
	}
	child := &domain.Prompt{
		ID: domain.NewID(), Title: "Godot", Extends: base.ID,
		Blocks: []domain.Block{
			{Type: domain.BlockRules, Enabled: true, Content: "Usa GDScript idiomático."},
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Schema: 1,
	}
	if _, _, err := s.WS.SavePrompt(base, fsrepo.DirTemplates+"/base-coding.md"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.WS.SavePrompt(child, fsrepo.DirTemplates+"/godot.md"); err != nil {
		t.Fatal(err)
	}

	p, _, err := s.CreatePrompt(CreateOpts{Title: "Juego plataformas", TemplateID: child.ID})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := p.Block(domain.BlockRules); b.Content != "Usa GDScript idiomático." {
		t.Fatalf("override de plantilla hija falló: %q", b.Content)
	}
	if b, ok := p.Block(domain.BlockRole); !ok || !strings.Contains(b.Content, "{{style}}") {
		t.Fatal("bloque heredado del padre ausente")
	}
	if p.Variables["style"] != "clean" {
		t.Fatal("variables de plantilla no heredadas")
	}
	if p.Category != "software-development" {
		t.Fatalf("categoría heredada: %q", p.Category)
	}
}

func TestRenderWithWorkspaceVars(t *testing.T) {
	s := newStudio(t)
	s.WS.SaveVariables("global", map[string]string{"language": "Go"})
	p, rel, _ := s.CreatePrompt(CreateOpts{Title: "Render"})
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "Escribe {{language}} idiomático."})
	if _, err := s.Save(p, rel); err != nil {
		t.Fatal(err)
	}
	out, err := s.Render(string(p.ID), "", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Escribe Go idiomático.") {
		t.Fatalf("render: %q", out)
	}
	// Strict con variable sin definir falla.
	p.SetBlock(domain.Block{Type: domain.BlockContext, Enabled: true, Content: "{{nope}}"})
	s.Save(p, rel)
	if _, err := s.Render(string(p.ID), "", true); !errors.Is(err, domain.ErrUnresolvedVars) {
		t.Fatalf("esperaba ErrUnresolvedVars: %v", err)
	}
}

func TestValidatePersistsScore(t *testing.T) {
	s := newStudio(t)
	p, _, _ := s.CreatePrompt(CreateOpts{Title: "Validar"})
	report, err := s.Validate(string(p.ID), "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Score >= 100 {
		t.Fatal("un esqueleto vacío no puede puntuar 100")
	}
	hits, _ := s.List(sqlindex.ListFilter{})
	if hits[0].Score == nil || *hits[0].Score != report.Score {
		t.Fatal("la puntuación no se persistió en el índice")
	}
}

func TestExportFormats(t *testing.T) {
	s := newStudio(t)
	s.WS.SaveVariables("global", map[string]string{"x": "valor"})
	p, rel, _ := s.CreatePrompt(CreateOpts{Title: "Exportable"})
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "Objetivo con {{x}}."})
	s.Save(p, rel)

	for format, want := range map[string]string{
		"md":   "## Objective",
		"txt":  "Objetivo con valor.",
		"json": `"title": "Exportable"`,
		"yaml": "title: Exportable",
	} {
		data, ext, err := s.Export(string(p.ID), format, "")
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s: falta %q en:\n%s", format, want, data)
		}
		if ext == "" {
			t.Errorf("%s: extensión vacía", format)
		}
	}
	if _, _, err := s.Export(string(p.ID), "docx", ""); err == nil {
		t.Fatal("formato desconocido debe fallar")
	}
}

func TestImportRoundTripJSONAndYAML(t *testing.T) {
	s := newStudio(t)
	p, rel, _ := s.CreatePrompt(CreateOpts{Title: "Original único"})
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "contenido de ida y vuelta"})
	s.Save(p, rel)

	for _, format := range []string{"json", "yaml"} {
		data, _, err := s.Export(string(p.ID), format, "")
		if err != nil {
			t.Fatal(err)
		}
		s2 := newStudio(t) // workspace limpio
		imported, _, err := s2.Import(data, "")
		if err != nil {
			t.Fatalf("import %s: %v", format, err)
		}
		if imported.Title != "Original único" {
			t.Fatalf("import %s alteró el título: %q", format, imported.Title)
		}
		b, _ := imported.Block(domain.BlockObjective)
		if b.Content != "contenido de ida y vuelta" {
			t.Fatalf("import %s alteró el contenido: %q", format, b.Content)
		}
	}
}

func TestImportPlainTextFallback(t *testing.T) {
	s := newStudio(t)
	p, _, err := s.Import([]byte("un prompt suelto copiado de un chat"), "Copiado")
	if err != nil {
		t.Fatal(err)
	}
	b, ok := p.Block(domain.BlockContext)
	if !ok || !strings.Contains(b.Content, "prompt suelto") {
		t.Fatalf("fallback de texto plano: %+v", p.Blocks)
	}
	if p.Title != "Copiado" {
		t.Fatalf("título: %q", p.Title)
	}
}

func TestSearchAfterCreate(t *testing.T) {
	s := newStudio(t)
	p, rel, _ := s.CreatePrompt(CreateOpts{Title: "Buscable"})
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "palabra especialisima aquí"})
	s.Save(p, rel)
	hits, err := s.Search("especialisima", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
}

func TestReindexRebuild(t *testing.T) {
	s := newStudio(t)
	s.CreatePrompt(CreateOpts{Title: "Uno"})
	s.CreatePrompt(CreateOpts{Title: "Dos"})
	stats, err := s.Reindex(true)
	if err != nil || stats.Indexed != 2 {
		t.Fatalf("rebuild: %+v err=%v", stats, err)
	}
}
