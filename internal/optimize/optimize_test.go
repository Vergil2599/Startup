package optimize

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func sidecarCmd(t *testing.T) []string {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 no disponible")
	}
	main, err := filepath.Abs("../../sidecar/pes_ai/main.py")
	if err != nil {
		t.Fatal(err)
	}
	return []string{py, main}
}

func TestSuggestAgainstRealSidecar(t *testing.T) {
	c, err := New(sidecarCmd(t))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	p := &domain.Prompt{ID: "x", Title: "Optimizable", Blocks: []domain.Block{
		{Type: domain.BlockObjective, Enabled: true, Content: "Por favor, ayudar al usuario con su código."},
		{Type: domain.BlockOutput, Enabled: true, Content: "Devuelve un JSON."},
	}}
	sugs, err := c.Suggest(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, s := range sugs {
		kinds[s.Kind] = true
	}
	// Espera: objetivo vago (precision), cortesía (directness), examples (structure).
	for _, want := range []string{"precision", "directness", "structure"} {
		if !kinds[want] {
			t.Errorf("falta sugerencia %q en %+v", want, sugs)
		}
	}
	// Las sugerencias jamás modifican el prompt.
	if b, _ := p.Block(domain.BlockObjective); b.Content != "Por favor, ayudar al usuario con su código." {
		t.Fatal("el optimizer no debe tocar el prompt")
	}
}

func TestCleanPromptNoSuggestions(t *testing.T) {
	c, err := New(sidecarCmd(t))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	p := &domain.Prompt{ID: "x", Title: "Limpio", Blocks: []domain.Block{
		{Type: domain.BlockObjective, Enabled: true, Content: "Detecta 3 defectos por revisión."},
	}}
	sugs, err := c.Suggest(context.Background(), p)
	if err != nil || len(sugs) != 0 {
		t.Fatalf("sugs=%+v err=%v", sugs, err)
	}
}

func TestGracefulWhenSidecarMissing(t *testing.T) {
	if _, err := New([]string{"/ruta/que/no/existe"}); err == nil {
		t.Fatal("sidecar inexistente debe dar error limpio, no pánico")
	}
	t.Setenv("PES_AI_CMD", "")
	t.Setenv("PATH", t.TempDir()) // sin pes-ai ni python3 en PATH
	// DetectCommand en un cwd sin sidecar/ debe fallar limpiamente.
	dir := t.TempDir()
	t.Chdir(dir)
	if _, err := DetectCommand(); err == nil {
		t.Fatal("sin sidecar disponible debe devolver error explicativo")
	}
}
