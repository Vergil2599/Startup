package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/validation"
)

// examplePluginDir localiza el plugin de ejemplo del repositorio.
func examplePluginDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../../plugins/example-python")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugin.toml")); err != nil {
		t.Skipf("plugin de ejemplo no encontrado: %v", err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 no disponible")
	}
	return dir
}

func startExample(t *testing.T) *Proc {
	t.Helper()
	dir := examplePluginDir(t)
	m, err := LoadManifest(filepath.Join(dir, "plugin.toml"))
	if err != nil {
		t.Fatal(err)
	}
	proc, err := Start(Discovered{Manifest: m, Dir: dir, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proc.Stop() })
	return proc
}

func TestManifestValidation(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		p := filepath.Join(dir, "plugin.toml")
		os.WriteFile(p, []byte(content), 0o644)
		return p
	}
	good := `[plugin]
id = "a.b"
name = "X"
version = "1.0.0"
api = "1"
entry = ["python3", "main.py"]
[contributes]
extension_points = ["validation.rule"]
`
	if _, err := LoadManifest(write(good)); err != nil {
		t.Fatalf("manifiesto válido rechazado: %v", err)
	}
	bad := map[string]string{
		"id inválido":    strings.Replace(good, `id = "a.b"`, `id = "A B!"`, 1),
		"api incompat.":  strings.Replace(good, `api = "1"`, `api = "9"`, 1),
		"sin entry":      strings.Replace(good, `entry = ["python3", "main.py"]`, `entry = []`, 1),
		"ext desconocido": strings.Replace(good, `["validation.rule"]`, `["magia.negra"]`, 1),
		"toml roto":      "[plugin\nid=",
	}
	for name, content := range bad {
		if _, err := LoadManifest(write(content)); err == nil {
			t.Errorf("%s: debería rechazarse", name)
		}
	}
}

func TestDiscoverRequiresConsent(t *testing.T) {
	ws := t.TempDir()
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

	ds, err := Discover(ws)
	if err != nil || len(ds) != 1 {
		t.Fatalf("ds=%v err=%v", ds, err)
	}
	if ds[0].Enabled {
		t.Fatal("un plugin recién descubierto NO debe estar habilitado")
	}
	if err := Enable(ws, "demo.plugin"); err != nil {
		t.Fatal(err)
	}
	ds, _ = Discover(ws)
	if !ds[0].Enabled {
		t.Fatal("Enable no surtió efecto")
	}
	Disable(ws, "demo.plugin")
	ds, _ = Discover(ws)
	if ds[0].Enabled {
		t.Fatal("Disable no surtió efecto")
	}
}

func TestDiscoverReportsBrokenManifest(t *testing.T) {
	ws := t.TempDir()
	pdir := filepath.Join(ws, ".pes", "plugins", "roto")
	os.MkdirAll(pdir, 0o755)
	os.WriteFile(filepath.Join(pdir, "plugin.toml"), []byte("basura ["), 0o644)
	ds, err := Discover(ws)
	if err != nil || len(ds) != 1 || ds[0].LoadErr == "" {
		t.Fatalf("ds=%+v err=%v", ds, err)
	}
}

// ── Tests de contrato contra el plugin Python real ──────────────────────────

func TestContractDescribe(t *testing.T) {
	proc := startExample(t)
	d, err := proc.Describe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "dev.pes.example-python" || len(d.ExtensionPoints) != 2 {
		t.Fatalf("describe=%+v", d)
	}
}

func TestContractValidationRule(t *testing.T) {
	proc := startExample(t)
	rule := NewRuleAdapter(proc)
	long := strings.Repeat("palabra ", 400)
	p := &domain.Prompt{ID: "x", Title: "T", Blocks: []domain.Block{
		{Type: domain.BlockContext, Enabled: true, Content: long},
		{Type: domain.BlockNotes, Enabled: false, Content: long}, // desactivado: no cuenta
	}}
	findings := rule.Check(validation.Input{Prompt: p})
	if len(findings) != 1 || findings[0].Severity != domain.SeverityWarn {
		t.Fatalf("findings=%+v", findings)
	}
	if findings[0].BlockType != domain.BlockContext {
		t.Fatalf("bloque señalado: %v", findings[0].BlockType)
	}
	// La regla del plugin se integra en el engine completo.
	engine := validation.NewEngine()
	engine.Register(rule)
	report := engine.Validate(validation.Input{Prompt: p})
	found := false
	for _, f := range report.Findings {
		if f.RuleID == rule.ID() {
			found = true
		}
	}
	if !found {
		t.Fatal("la regla del plugin no aparece en el informe del engine")
	}
}

func TestContractLLMProvider(t *testing.T) {
	proc := startExample(t)
	provider := NewProviderAdapter(proc)
	resp, err := provider.Complete(context.Background(), ai.Request{Prompt: "hola plugin"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text, "eco desde plugin") || !strings.Contains(resp.Text, "hola plugin") {
		t.Fatalf("resp=%q", resp.Text)
	}
	if provider.Name() != "plugin:dev.pes.example-python" {
		t.Fatalf("name=%q", provider.Name())
	}
}

func TestContractUnknownMethodAndRecovery(t *testing.T) {
	proc := startExample(t)
	var out map[string]any
	if err := proc.Call(context.Background(), "metodo.inexistente", nil, &out); err == nil {
		t.Fatal("método desconocido debe dar error JSON-RPC")
	}
	// El proceso sigue vivo y funcional tras el error.
	if _, err := proc.Describe(context.Background()); err != nil {
		t.Fatalf("el plugin debe sobrevivir a un error: %v", err)
	}
}

func TestCircuitBreakerSuspends(t *testing.T) {
	// Plugin que muere al instante: cada llamada falla.
	ws := t.TempDir()
	pdir := filepath.Join(ws, "dead")
	os.MkdirAll(pdir, 0o755)
	m := &Manifest{}
	m.Plugin.ID = "dead.plugin"
	m.Plugin.API = APIVersion
	m.Plugin.Entry = []string{"true"} // sale sin responder
	m.Contributes.ExtensionPoints = []string{ExtLLMProvider}
	proc, err := Start(Discovered{Manifest: m, Dir: pdir})
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Stop()
	proc.CallTimeout = 200 * time.Millisecond
	for i := 0; i < MaxFailures; i++ {
		if err := proc.Call(context.Background(), "x", nil, nil); err == nil {
			t.Fatal("debería fallar")
		}
	}
	if !proc.Suspended() {
		t.Fatal("tras MaxFailures el plugin debe quedar suspendido")
	}
	if err := proc.Call(context.Background(), "x", nil, nil); err == nil ||
		!strings.Contains(err.Error(), "suspendido") {
		t.Fatalf("las llamadas a un plugin suspendido se rechazan: %v", err)
	}
}
