// Package optimize es el cliente Go del sidecar pes-ai (Optimizer OPCIONAL).
// Si el sidecar no está instalado o falla el handshake, Available() es false
// y el resto de PES funciona exactamente igual: la IA nunca es una dependencia.
package optimize

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/plugin"
)

// Suggestion es una propuesta de mejora. Nunca se aplica automáticamente.
type Suggestion struct {
	BlockType       string `json:"block_type"`
	Kind            string `json:"kind"` // redundancy | clarity | precision | directness | structure
	Message         string `json:"message"`
	Replacement     string `json:"replacement,omitempty"`
	ReplacementHint string `json:"replacement_hint,omitempty"`
}

// Client gestiona el proceso del sidecar.
type Client struct {
	proc *plugin.Proc
}

// DetectCommand localiza el sidecar: $PES_AI_CMD, `pes-ai` en PATH, o el
// árbol de fuentes (desarrollo). Devuelve error si no hay ninguno.
func DetectCommand() ([]string, error) {
	if cmd := os.Getenv("PES_AI_CMD"); cmd != "" {
		return []string{cmd}, nil
	}
	if path, err := exec.LookPath("pes-ai"); err == nil {
		return []string{path}, nil
	}
	// Modo desarrollo: sidecar/pes_ai/main.py junto al binario o al cwd.
	for _, base := range []string{".", executableDir()} {
		candidate := filepath.Join(base, "sidecar", "pes_ai", "main.py")
		if _, err := os.Stat(candidate); err == nil {
			if py, perr := exec.LookPath("python3"); perr == nil {
				return []string{py, candidate}, nil
			}
		}
	}
	return nil, fmt.Errorf("pes-ai no está instalado (opcional): la optimización asistida no estará disponible")
}

// New arranca el sidecar con el comando dado y verifica el handshake.
func New(command []string) (*Client, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("comando del sidecar vacío")
	}
	m := &plugin.Manifest{}
	m.Plugin.ID = "dev.pes.ai"
	m.Plugin.Name = "pes-ai"
	m.Plugin.API = plugin.APIVersion
	m.Plugin.Entry = command
	m.Contributes.ExtensionPoints = []string{plugin.ExtOptimizer}
	proc, err := plugin.Start(plugin.Discovered{Manifest: m, Dir: "."})
	if err != nil {
		return nil, err
	}
	c := &Client{proc: proc}
	ctx, cancel := context.WithTimeout(context.Background(), plugin.HandshakeTimeout)
	defer cancel()
	desc, err := proc.Describe(ctx)
	if err != nil {
		proc.Stop()
		return nil, fmt.Errorf("handshake con pes-ai falló: %w", err)
	}
	ok := false
	for _, ep := range desc.ExtensionPoints {
		if ep == plugin.ExtOptimizer {
			ok = true
		}
	}
	if !ok {
		proc.Stop()
		return nil, fmt.Errorf("el proceso no contribuye el extension point optimizer")
	}
	return c, nil
}

// Suggest pide sugerencias de mejora para un prompt.
func (c *Client) Suggest(ctx context.Context, p *domain.Prompt) ([]Suggestion, error) {
	var out struct {
		Suggestions []Suggestion `json:"suggestions"`
	}
	err := c.proc.Call(ctx, "optimizer.suggest", map[string]any{"prompt": plugin.ToIR(p)}, &out)
	if err != nil {
		return nil, err
	}
	return out.Suggestions, nil
}

// Close termina el sidecar.
func (c *Client) Close() error { return c.proc.Stop() }

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
