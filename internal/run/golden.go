package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// Golden es la respuesta de referencia de un prompt: sirve para detectar
// regresiones ("deriva del prompt") cuando el prompt o el modelo cambian.
type Golden struct {
	PromptID   string    `json:"prompt_id"`
	PromptHash string    `json:"prompt_hash"` // hash del render con el que se grabó
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Response   string    `json:"response"`
	CreatedAt  time.Time `json:"created_at"`
}

// CheckResult es el veredicto de comparar una respuesta nueva con la golden.
type CheckResult struct {
	Similarity    float64 `json:"similarity"` // Jaccard 0..1
	PromptChanged bool    `json:"prompt_changed"`
	Pass          bool    `json:"pass"`
	Threshold     float64 `json:"threshold"`
}

func (s *Service) goldenPath(promptID domain.ID) string {
	return filepath.Join(s.ws.Root, fsrepo.DirMeta, "golden", string(promptID)+".json")
}

// SaveGolden graba un resultado como referencia del prompt.
func (s *Service) SaveGolden(r Result) error {
	if r.Error != "" {
		return fmt.Errorf("no se puede grabar como golden un run con error: %s", r.Error)
	}
	g := Golden{
		PromptID: r.PromptID, PromptHash: r.PromptHash,
		Provider: r.Provider, Model: r.Model, Response: r.Response,
		CreatedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return fsrepo.WriteFileAtomic(s.goldenPath(domain.ID(r.PromptID)), data)
}

// LoadGolden carga la referencia de un prompt.
func (s *Service) LoadGolden(promptID domain.ID) (Golden, error) {
	data, err := os.ReadFile(s.goldenPath(promptID))
	if err != nil {
		if os.IsNotExist(err) {
			return Golden{}, fmt.Errorf("%w: no hay golden para este prompt (usa `pes golden set`)", domain.ErrNotFound)
		}
		return Golden{}, err
	}
	var g Golden
	if err := json.Unmarshal(data, &g); err != nil {
		return Golden{}, err
	}
	return g, nil
}

// CheckGolden compara una respuesta nueva con la referencia.
func (s *Service) CheckGolden(promptID domain.ID, newResult Result, threshold float64) (CheckResult, error) {
	if threshold <= 0 {
		threshold = 0.6
	}
	g, err := s.LoadGolden(promptID)
	if err != nil {
		return CheckResult{}, err
	}
	if newResult.Error != "" {
		return CheckResult{Threshold: threshold}, fmt.Errorf("el run a comparar falló: %s", newResult.Error)
	}
	sim := jaccard(wordSet(g.Response), wordSet(newResult.Response))
	return CheckResult{
		Similarity:    sim,
		PromptChanged: g.PromptHash != newResult.PromptHash,
		Pass:          sim >= threshold,
		Threshold:     threshold,
	}, nil
}

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		m[w] = true
	}
	return m
}
