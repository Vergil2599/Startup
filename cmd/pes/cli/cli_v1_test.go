package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryAndRestoreFlow(t *testing.T) {
	ws := t.TempDir()
	run(t, "init", "-w", ws)
	out, _ := run(t, "new", "Con historia", "-w", ws)
	id := idRe.FindStringSubmatch(out)[1]

	out, err := run(t, "snapshot", id, "-w", ws, "-m", "primera")
	if err != nil || !strings.Contains(out, "v1 creada") {
		t.Fatalf("snapshot: %v\n%s", err, out)
	}
	// Editar y snapshot de nuevo.
	rel := filepath.Join(ws, "prompts", "con-historia.md")
	data, _ := os.ReadFile(rel)
	os.WriteFile(rel, []byte(strings.Replace(string(data), "## @objective\n", "## @objective\nnuevo contenido\n", 1)), 0o644)
	run(t, "snapshot", id, "-w", ws, "-m", "segunda")

	out, err = run(t, "history", id, "-w", ws)
	if err != nil || !strings.Contains(out, "v1") || !strings.Contains(out, "segunda") {
		t.Fatalf("history: %v\n%s", err, out)
	}
	out, err = run(t, "restore", id, "v1", "-w", ws)
	if err != nil || !strings.Contains(out, "restaurado") {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	restored, _ := os.ReadFile(rel)
	if strings.Contains(string(restored), "nuevo contenido") {
		t.Fatal("restore no volvió a v1")
	}
}

func TestDiffCommand(t *testing.T) {
	ws := t.TempDir()
	run(t, "init", "-w", ws)
	oa, _ := run(t, "new", "Alfa", "-w", ws)
	ob, _ := run(t, "new", "Beta", "-w", ws)
	a, b := idRe.FindStringSubmatch(oa)[1], idRe.FindStringSubmatch(ob)[1]
	rel := filepath.Join(ws, "prompts", "beta.md")
	data, _ := os.ReadFile(rel)
	os.WriteFile(rel, []byte(strings.Replace(string(data), "## @objective\n", "## @objective\ncontenido beta\n", 1)), 0o644)

	out, err := run(t, "diff", a, b, "-w", ws)
	if err != nil || !strings.Contains(out, "modified") || !strings.Contains(out, "+ contenido beta") {
		t.Fatalf("diff: %v\n%s", err, out)
	}
}

func TestComposeCommand(t *testing.T) {
	ws := t.TempDir()
	run(t, "init", "-w", ws)
	oa, _ := run(t, "new", "Base capa", "-w", ws)
	a := idRe.FindStringSubmatch(oa)[1]
	comp := filepath.Join(t.TempDir(), "comp.yaml")
	os.WriteFile(comp, []byte("title: Compuesto final\nstrategy: concat\nlayers:\n  - {ref: "+a+", enabled: true}\n"), 0o644)
	out, err := run(t, "compose", comp, "-w", ws)
	if err != nil || !strings.Contains(out, "compuesto") {
		t.Fatalf("compose: %v\n%s", err, out)
	}
	if lst, _ := run(t, "list", "-w", ws); !strings.Contains(lst, "Compuesto final") {
		t.Fatalf("el compuesto no está en la biblioteca: %s", lst)
	}
}

func TestRunCommandWithMock(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "hola desde mock"}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer llm.Close()
	ws := t.TempDir()
	run(t, "init", "-w", ws)
	o, _ := run(t, "new", "Ejecutable", "-w", ws)
	id := idRe.FindStringSubmatch(o)[1]
	os.WriteFile(filepath.Join(ws, ".pes", "providers.yaml"),
		[]byte("providers:\n  - {name: mock, type: openai, base_url: \""+llm.URL+"\", model: m}\n"), 0o644)

	out, err := run(t, "run", id, "-w", ws)
	if err != nil || !strings.Contains(out, "hola desde mock") {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if _, err := run(t, "run", id, "-w", ws, "-P", "inexistente"); err == nil {
		t.Fatal("proveedor inexistente debe fallar")
	}
}

func TestOptimizeCommand(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 no disponible")
	}
	main, _ := filepath.Abs("../../../sidecar/pes_ai/main.py")
	if _, err := os.Stat(main); err != nil {
		t.Skip("sidecar no encontrado")
	}
	t.Setenv("PES_AI_CMD", "") // forzar detección por árbol de fuentes no aplica: usar env
	py, _ := exec.LookPath("python3")
	_ = py
	// DetectCommand soporta PES_AI_CMD con un solo elemento; usamos un wrapper.
	wrapper := filepath.Join(t.TempDir(), "pes-ai")
	os.WriteFile(wrapper, []byte("#!/bin/sh\nexec python3 "+main+" \"$@\"\n"), 0o755)
	t.Setenv("PES_AI_CMD", wrapper)

	ws := t.TempDir()
	run(t, "init", "-w", ws)
	o, _ := run(t, "new", "Optimizable", "-w", ws)
	id := idRe.FindStringSubmatch(o)[1]
	rel := filepath.Join(ws, "prompts", "optimizable.md")
	data, _ := os.ReadFile(rel)
	os.WriteFile(rel, []byte(strings.Replace(string(data), "## @objective\n",
		"## @objective\nPor favor, ayudar al usuario.\n", 1)), 0o644)

	out, err := run(t, "optimize", id, "-w", ws)
	if err != nil {
		t.Fatalf("optimize: %v\n%s", err, out)
	}
	if !strings.Contains(out, "directness") && !strings.Contains(out, "precision") {
		t.Fatalf("sin sugerencias esperadas:\n%s", out)
	}
}

func TestBackupCommand(t *testing.T) {
	ws := t.TempDir()
	run(t, "init", "-w", ws)
	run(t, "new", "Respaldable", "-w", ws)
	out, err := run(t, "backup", "-w", ws)
	if err != nil || !strings.Contains(out, "backup creado") {
		t.Fatalf("backup: %v\n%s", err, out)
	}
	out, err = run(t, "backup", "-w", ws, "--passphrase", "secreta")
	if err != nil || !strings.Contains(out, ".age") {
		t.Fatalf("backup cifrado: %v\n%s", err, out)
	}
}

func TestPluginCommands(t *testing.T) {
	ws := t.TempDir()
	run(t, "init", "-w", ws)
	out, err := run(t, "plugin", "list", "-w", ws)
	if err != nil || !strings.Contains(out, "sin plugins") {
		t.Fatalf("plugin list vacío: %v\n%s", err, out)
	}
	pdir := filepath.Join(ws, ".pes", "plugins", "demo")
	os.MkdirAll(pdir, 0o755)
	os.WriteFile(filepath.Join(pdir, "plugin.toml"), []byte(`[plugin]
id = "demo.plugin"
name = "Demo"
version = "0.1"
api = "1"
entry = ["true"]
[contributes]
extension_points = ["llm.provider"]
`), 0o644)
	out, _ = run(t, "plugin", "list", "-w", ws)
	if !strings.Contains(out, "deshabilitado") {
		t.Fatalf("plugin nuevo debe estar deshabilitado:\n%s", out)
	}
	run(t, "plugin", "enable", "demo.plugin", "-w", ws)
	out, _ = run(t, "plugin", "list", "-w", ws)
	if !strings.Contains(out, "● demo.plugin") {
		t.Fatalf("enable no reflejado:\n%s", out)
	}
}
