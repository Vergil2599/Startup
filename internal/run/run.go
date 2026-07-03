// Package run implementa el Testing Engine de PES: el simulador que ejecuta
// un prompt renderizado contra uno o varios proveedores/modelos en paralelo
// y persiste los resultados en .pes/runs/.
package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// Target es un destino de ejecución.
type Target struct {
	Provider ai.Provider
	Model    string // vacío = modelo por defecto del proveedor
}

// Params son los parámetros de generación comunes.
type Params struct {
	Temperature float64
	MaxTokens   int
}

// Result es el resultado persistible de una ejecución.
type Result struct {
	ID         string    `json:"id"`
	PromptID   string    `json:"prompt_id"`
	PromptHash string    `json:"prompt_hash"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Params     Params    `json:"params"`
	Response   string    `json:"response,omitempty"`
	Error      string    `json:"error,omitempty"`
	LatencyMS  int64     `json:"latency_ms"`
	TokensIn   int       `json:"tokens_in"`
	TokensOut  int       `json:"tokens_out"`
	CreatedAt  time.Time `json:"created_at"`
}

// Service ejecuta y persiste runs dentro de un workspace.
type Service struct {
	ws          *fsrepo.Workspace
	Concurrency int // máximo de ejecuciones simultáneas (por defecto 4)
}

// NewService crea el servicio de ejecución.
func NewService(ws *fsrepo.Workspace) *Service { return &Service{ws: ws, Concurrency: 4} }

// Run ejecuta el prompt renderizado contra todos los targets en paralelo.
// Un fallo en un proveedor no aborta el resto; queda registrado en su Result.
func (s *Service) Run(ctx context.Context, promptID domain.ID, rendered string, targets []Target, params Params) ([]Result, error) {
	sem := make(chan struct{}, max(1, s.Concurrency))
	results := make([]Result, len(targets))
	var wg sync.WaitGroup
	hash := fsrepo.HashBytes([]byte(rendered))
	for i, tgt := range targets {
		wg.Add(1)
		go func(i int, tgt Target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r := Result{
				ID: string(domain.NewID()), PromptID: string(promptID), PromptHash: hash,
				Provider: tgt.Provider.Name(), Model: tgt.Model, Params: params,
				CreatedAt: time.Now().UTC(),
			}
			resp, err := tgt.Provider.Complete(ctx, ai.Request{
				Model: tgt.Model, Prompt: rendered,
				Temperature: params.Temperature, MaxTokens: params.MaxTokens,
			})
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Response = resp.Text
				r.LatencyMS = resp.Latency.Milliseconds()
				r.TokensIn, r.TokensOut = resp.TokensIn, resp.TokensOut
			}
			results[i] = r
		}(i, tgt)
	}
	wg.Wait()
	for _, r := range results {
		if err := s.persist(r); err != nil {
			return results, err
		}
	}
	return results, nil
}

func (s *Service) runsDir() string {
	return filepath.Join(s.ws.Root, fsrepo.DirMeta, "runs")
}

func (s *Service) persist(r Result) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return fsrepo.WriteFileAtomic(filepath.Join(s.runsDir(), r.ID+".json"), data)
}

// History devuelve las ejecuciones guardadas de un prompt (recientes primero).
func (s *Service) History(promptID domain.ID, limit int) ([]Result, error) {
	entries, err := os.ReadDir(s.runsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Result
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(s.runsDir(), e.Name()))
		if rerr != nil {
			continue
		}
		var r Result
		if json.Unmarshal(data, &r) != nil {
			continue
		}
		if promptID == "" || r.PromptID == string(promptID) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
