package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// run ejecuta la CLI con los argumentos dados y devuelve stdout.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := Root()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

var idRe = regexp.MustCompile(`id=([0-9A-Z]{26})`)

func TestFullCLIFlow(t *testing.T) {
	ws := t.TempDir()

	// init
	out, err := run(t, "init", "-w", ws)
	if err != nil || !strings.Contains(out, "workspace inicializado") {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// comandos sobre un dir sin init deben fallar con mensaje claro
	if _, err := run(t, "list", "-w", t.TempDir()); err == nil || !strings.Contains(err.Error(), "pes init") {
		t.Fatalf("sin init debe sugerir `pes init`: %v", err)
	}

	// new
	out, err = run(t, "new", "Revisor de Go", "-w", ws, "-c", "software-development", "--tags", "go,review")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	m := idRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("new no devolvió id: %s", out)
	}
	id := m[1]

	// editar el archivo a mano (fuente de verdad legible) y que todo lo recoja
	relPath := filepath.Join(ws, "prompts", "revisor-de-go.md")
	data, err := os.ReadFile(relPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "## @objective\n",
		"## @objective\nDetectar al menos 3 defectos en código {{language}}.\n", 1)
	edited = strings.Replace(edited, "## @output\n",
		"## @output\nLista Markdown con archivo y línea.\n", 1)
	edited = strings.Replace(edited, "## @role\n",
		"## @role\nEres revisor senior.\n", 1)
	if err := os.WriteFile(relPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// variables globales
	if err := os.WriteFile(filepath.Join(ws, "variables", "global.yaml"),
		[]byte("language: Go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// render con variable resuelta
	out, err = run(t, "render", id, "-w", ws, "--strict")
	if err != nil || !strings.Contains(out, "defectos en código Go.") {
		t.Fatalf("render: %v\n%s", err, out)
	}

	// validate: prompt completo puntúa alto
	out, err = run(t, "validate", id, "-w", ws)
	if err != nil || !strings.Contains(out, "puntuación:") {
		t.Fatalf("validate: %v\n%s", err, out)
	}

	// validate --min-score como gate de CI
	if _, err := run(t, "validate", id, "-w", ws, "--min-score", "101"); err == nil {
		t.Fatal("min-score imposible debe fallar")
	}

	// search encuentra por contenido editado externamente
	out, err = run(t, "search", "defectos", "-w", ws)
	if err != nil || !strings.Contains(out, "Revisor de Go") {
		t.Fatalf("search: %v\n%s", err, out)
	}

	// list con filtro de tag
	out, err = run(t, "list", "-w", ws, "--tag", "go")
	if err != nil || !strings.Contains(out, "Revisor de Go") {
		t.Fatalf("list: %v\n%s", err, out)
	}

	// export a JSON y reimport en otro workspace
	exportPath := filepath.Join(t.TempDir(), "prompt.json")
	if _, err := run(t, "export", id, "-w", ws, "-f", "json", "-o", exportPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	ws2 := t.TempDir()
	run(t, "init", "-w", ws2)
	out, err = run(t, "import", exportPath, "-w", ws2)
	if err != nil || !strings.Contains(out, "importado") {
		t.Fatalf("import: %v\n%s", err, out)
	}
	out, _ = run(t, "list", "-w", ws2)
	if !strings.Contains(out, "Revisor de Go") {
		t.Fatalf("el prompt importado no aparece: %s", out)
	}

	// reindex --rebuild
	out, err = run(t, "reindex", "-w", ws, "--rebuild")
	if err != nil || !strings.Contains(out, "indexados=1") {
		t.Fatalf("reindex: %v\n%s", err, out)
	}

	// export a stdout en todos los formatos
	for _, f := range []string{"md", "txt", "json", "yaml"} {
		if out, err := run(t, "export", id, "-w", ws, "-f", f); err != nil || out == "" {
			t.Fatalf("export %s: %v", f, err)
		}
	}
	if _, err := run(t, "export", id, "-w", ws, "-f", "docx"); err == nil {
		t.Fatal("formato desconocido debe fallar")
	}
}

func TestTemplateFlow(t *testing.T) {
	ws := t.TempDir()
	run(t, "init", "-w", ws)

	// Crear una plantilla a mano (archivo en templates/).
	tpl := `---
pes: 1
id: 01TPLBASE00000000000000000
title: Base coding
category: software-development
variables:
  style: limpio
blocks:
  - {type: role, enabled: true}
  - {type: rules, enabled: true}
---

## @role
Eres un ingeniero de software {{style}}.

## @rules
Sigue SOLID.
`
	if err := os.WriteFile(filepath.Join(ws, "templates", "base.md"), []byte(tpl), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "new", "Desde plantilla", "-w", ws, "-t", "01TPLBASE00000000000000000")
	if err != nil {
		t.Fatalf("new -t: %v\n%s", err, out)
	}
	id := idRe.FindStringSubmatch(out)[1]
	rendered, err := run(t, "render", id, "-w", ws)
	if err != nil || !strings.Contains(rendered, "ingeniero de software limpio") {
		t.Fatalf("render de instancia: %v\n%s", err, rendered)
	}
	if !strings.Contains(rendered, "Sigue SOLID.") {
		t.Fatalf("bloque heredado ausente: %s", rendered)
	}
}
