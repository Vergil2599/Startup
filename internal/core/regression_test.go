package core

// Tests de regresión de los hallazgos de las pruebas exploratorias
// (simulación de usuarios). Cada test corresponde a un bug real encontrado.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
)

// BUG #1: dos prompts con el mismo título sobrescribían el mismo archivo.
func TestDuplicateTitleNeverOverwrites(t *testing.T) {
	s := newStudio(t)
	p1, rel1, err := s.CreatePrompt(CreateOpts{Title: "Repetido"})
	if err != nil {
		t.Fatal(err)
	}
	p2, rel2, err := s.CreatePrompt(CreateOpts{Title: "Repetido"})
	if err != nil {
		t.Fatal(err)
	}
	if rel1 == rel2 {
		t.Fatalf("mismo archivo para dos prompts: %s", rel1)
	}
	if !strings.Contains(rel2, "repetido-2") {
		t.Fatalf("ruta única esperada: %s", rel2)
	}
	// Ambos sobreviven y son recuperables.
	for _, id := range []domain.ID{p1.ID, p2.ID} {
		if _, err := s.Get(string(id)); err != nil {
			t.Fatalf("prompt %s perdido: %v", id, err)
		}
	}
	hits, _ := s.List(sqlindex.ListFilter{})
	if len(hits) != 2 {
		t.Fatalf("biblioteca debe tener 2, tiene %d", len(hits))
	}
	// Guardar de nuevo el mismo prompt sin ruta NO debe crear un tercero.
	if _, err := s.Save(p1, ""); err != nil {
		t.Fatal(err)
	}
	hits, _ = s.List(sqlindex.ListFilter{})
	if len(hits) != 2 {
		t.Fatalf("re-guardar duplicó el prompt: %d", len(hits))
	}
	_ = p2
}

// BUG #1b: títulos con slug vacío ("!!!") colisionaban en prompts/prompt.md.
func TestEmptySlugTitlesGetUniquePaths(t *testing.T) {
	s := newStudio(t)
	_, rel1, _ := s.CreatePrompt(CreateOpts{Title: "!!!"})
	_, rel2, _ := s.CreatePrompt(CreateOpts{Title: "???"})
	if rel1 == rel2 {
		t.Fatalf("colisión de slug vacío: %s", rel1)
	}
}

// BUG #2: importar un export propio conservaba el ID y machacaba el original.
func TestImportExistingIDCreatesNewPrompt(t *testing.T) {
	s := newStudio(t)
	p, rel, _ := s.CreatePrompt(CreateOpts{Title: "Original"})
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "contenido original"})
	s.Save(p, rel)
	data, _, err := s.Export(string(p.ID), "json", "")
	if err != nil {
		t.Fatal(err)
	}
	imported, irel, err := s.Import(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if imported.ID == p.ID {
		t.Fatal("el import debe asignar un ID nuevo si ya existe")
	}
	if irel == rel {
		t.Fatal("el import no debe pisar el archivo original")
	}
	hits, _ := s.List(sqlindex.ListFilter{})
	if len(hits) != 2 {
		t.Fatalf("deben existir original + importado: %d", len(hits))
	}
	// El original está intacto.
	e, _ := s.Get(string(p.ID))
	b, _ := e.Prompt.Block(domain.BlockObjective)
	if b.Content != "contenido original" {
		t.Fatal("el original fue modificado por el import")
	}
}

// BUG #5: un nombre de proyecto de variables inexistente pasaba en silencio.
func TestUnknownProjectVariablesFailsLoud(t *testing.T) {
	s := newStudio(t)
	p, _, _ := s.CreatePrompt(CreateOpts{Title: "Con proyecto"})
	if _, err := s.Render(string(p.ID), "proyecto-fantasma", false); err == nil ||
		!strings.Contains(err.Error(), "proyecto-fantasma") {
		t.Fatalf("proyecto inexistente debe fallar claro: %v", err)
	}
	// Con el archivo creado, funciona.
	s.WS.SaveVariables("real", map[string]string{"x": "1"})
	if _, err := s.Render(string(p.ID), "real", false); err != nil {
		t.Fatalf("proyecto existente: %v", err)
	}
}

// BUG #3: reemplazar un archivo con otro ID en la misma ruta rompía el índice
// (UNIQUE constraint) hasta reconstruirlo.
func TestIndexSurvivesFileReplacedWithNewID(t *testing.T) {
	s := newStudio(t)
	_, rel, _ := s.CreatePrompt(CreateOpts{Title: "Sustituible"})

	// Simular `git checkout`: mismo path, prompt distinto (otro ID).
	replacement := &domain.Prompt{
		ID: domain.NewID(), Title: "Sustituto", Schema: 1,
		Blocks: []domain.Block{{Type: domain.BlockObjective, Enabled: true, Content: "nuevo"}},
	}
	data, _ := fsrepo.Serialize(replacement)
	if err := os.WriteFile(filepath.Join(s.WS.Root, rel), data, 0o644); err != nil {
		t.Fatal(err)
	}

	stats, err := s.Reindex(false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Errors != 0 {
		t.Fatalf("el reemplazo no debe generar errores de índice: %+v", stats.ErrorDetails)
	}
	hits, _ := s.List(sqlindex.ListFilter{})
	if len(hits) != 1 || hits[0].Title != "Sustituto" {
		t.Fatalf("índice tras reemplazo: %+v", hits)
	}
	// Y las pasadas siguientes son estables (no flapping).
	stats, _ = s.Reindex(false)
	if stats.Errors != 0 || stats.Indexed != 0 {
		t.Fatalf("índice inestable tras reemplazo: %+v", stats)
	}
}

// BUG #4: los errores de reindexado eran anónimos.
func TestSyncErrorsIncludePaths(t *testing.T) {
	s := newStudio(t)
	s.CreatePrompt(CreateOpts{Title: "Sano"})
	os.WriteFile(filepath.Join(s.WS.Root, fsrepo.DirPrompts, "roto.md"), []byte("basura"), 0o644)
	stats, err := s.Reindex(false)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Errors != 1 || len(stats.ErrorDetails) != 1 ||
		!strings.Contains(stats.ErrorDetails[0], "roto.md") {
		t.Fatalf("el detalle debe nombrar al archivo: %+v", stats)
	}
	_ = errors.Is
}
