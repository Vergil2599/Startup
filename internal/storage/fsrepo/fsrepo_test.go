package fsrepo

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func fixture() *domain.Prompt {
	return &domain.Prompt{
		ID: "01J8ZK7Q3W9X2Y4V5B6N7M8P9R", Title: "Revisor de código Go",
		Description: "Revisión con estándares del equipo",
		Tags:        []string{"go", "code-review"}, Category: "software-development",
		Favorite:  true,
		Variables: map[string]string{"language": "Go"},
		Blocks: []domain.Block{
			{Type: domain.BlockRole, Enabled: true, Content: "Actúa como revisor senior de {{language}}."},
			{Type: domain.BlockObjective, Enabled: true, Content: "Detectar defectos.\n\nCon varias líneas."},
			{Type: domain.BlockConstraints, Enabled: false, Content: "No reescrituras completas."},
			{Type: domain.BlockNotes, Enabled: true, Content: ""}, // bloque vacío
		},
		CreatedAt: time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 2, 12, 30, 0, 0, time.UTC),
		Schema:    domain.SchemaVersion,
	}
}

func TestRoundTrip(t *testing.T) {
	p := fixture()
	data, err := Serialize(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("parse falló: %v\n---\n%s", err, data)
	}
	if !reflect.DeepEqual(p, got) {
		t.Fatalf("round-trip alteró el prompt:\nantes: %+v\ndespués: %+v", p, got)
	}
}

func TestSerializeIsHumanReadable(t *testing.T) {
	data, _ := Serialize(fixture())
	s := string(data)
	for _, want := range []string{"---\n", "title: Revisor de código Go", "## @role", "## @objective"} {
		if !strings.Contains(s, want) {
			t.Errorf("el archivo debe contener %q\n%s", want, s)
		}
	}
}

func TestParseToleratesExternalBlocks(t *testing.T) {
	// Un bloque añadido a mano en el cuerpo (sin declarar en el front-matter)
	// se conserva activo.
	data, _ := Serialize(fixture())
	edited := string(data) + "\n## @workflow\nPaso 1. Paso 2.\n"
	p, err := Parse([]byte(edited))
	if err != nil {
		t.Fatal(err)
	}
	b, ok := p.Block(domain.BlockWorkflow)
	if !ok || !b.Enabled || b.Content != "Paso 1. Paso 2." {
		t.Fatalf("bloque externo no conservado: %+v", b)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	bad := [][]byte{
		[]byte(""), []byte("sin front matter"),
		[]byte("---\ntitle: x\n(no cierra"),
		[]byte("---\npes: 1\n---\ncuerpo sin id ni title"),
		[]byte("---\nid: a\ntitle: x\nblocks: [{type: role, enabled: true}, {type: role, enabled: true}]\n---\n"),
	}
	for i, b := range bad {
		if _, err := Parse(b); err == nil {
			t.Errorf("caso %d: esperaba error de formato", i)
		}
	}
}

func TestWriteFileAtomicLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "a.md")
	if err := WriteFileAtomic(path, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "v2" {
		t.Fatalf("contenido=%q", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("quedaron archivos temporales: %v", entries)
	}
}

func TestWorkspaceSaveLoadDelete(t *testing.T) {
	ws, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := fixture()
	rel, hash, err := ws.SavePrompt(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if rel != filepath.Join("prompts", "revisor-de-codigo-go.md") {
		t.Fatalf("ruta derivada inesperada: %q", rel)
	}
	e, err := ws.LoadPrompt(rel)
	if err != nil {
		t.Fatal(err)
	}
	if e.ContentHash != hash || !reflect.DeepEqual(e.Prompt, p) {
		t.Fatal("load no coincide con save")
	}
	if err := ws.DeletePrompt(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.LoadPrompt(rel); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("esperaba ErrNotFound, obtuve %v", err)
	}
}

func TestWorkspaceOpenRequiresInit(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err == nil {
		t.Fatal("Open debe fallar sin `pes init`")
	}
	if _, err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err != nil {
		t.Fatalf("Open tras Init falló: %v", err)
	}
}

func TestWalkSkipsCorruptFiles(t *testing.T) {
	ws, _ := Init(t.TempDir())
	if _, _, err := ws.SavePrompt(fixture(), ""); err != nil {
		t.Fatal(err)
	}
	// Archivo corrupto en medio.
	os.WriteFile(filepath.Join(ws.Root, DirPrompts, "roto.md"), []byte("basura"), 0o644)
	var seen, errs int
	err := ws.Walk(DirPrompts, func(e *Entry) { seen++ }, func(path string, err error) { errs++ })
	if err != nil {
		t.Fatal(err)
	}
	if seen != 1 || errs != 1 {
		t.Fatalf("seen=%d errs=%d", seen, errs)
	}
}

func TestFindByID(t *testing.T) {
	ws, _ := Init(t.TempDir())
	p := fixture()
	ws.SavePrompt(p, "")
	e, err := ws.FindByID(p.ID)
	if err != nil || e.Prompt.Title != p.Title {
		t.Fatalf("FindByID: %v", err)
	}
	if _, err := ws.FindByID("no-existe"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("ID inexistente debe dar ErrNotFound")
	}
}

func TestVariablesRoundTripAndValidation(t *testing.T) {
	ws, _ := Init(t.TempDir())
	want := map[string]string{"project_name": "PES", "language": "Go"}
	if err := ws.SaveVariables("global", want); err != nil {
		t.Fatal(err)
	}
	got, err := ws.LoadVariables("global")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if got, err := ws.LoadVariables("inexistente"); err != nil || len(got) != 0 {
		t.Fatal("archivo ausente debe dar mapa vacío sin error")
	}
	if err := ws.SaveVariables("bad", map[string]string{"Not-Valid": "x"}); !errors.Is(err, domain.ErrInvalidVarName) {
		t.Fatalf("nombre inválido debe rechazarse: %v", err)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Revisor de código Go": "revisor-de-codigo-go",
		"  ¡Hola, Mundo!  ":    "hola-mundo",
		"":                     "prompt",
		"ñandú über":           "nandu-uber",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q)=%q, quería %q", in, got, want)
		}
	}
}
