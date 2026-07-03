package run

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// BenchSummary agrega las métricas de un target a lo largo de las repeticiones.
type BenchSummary struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Reps         int     `json:"reps"`
	Failures     int     `json:"failures"`
	LatencyP50MS int64   `json:"latency_p50_ms"`
	LatencyP95MS int64   `json:"latency_p95_ms"`
	TokensInAvg  float64 `json:"tokens_in_avg"`
	TokensOutAvg float64 `json:"tokens_out_avg"`
	// Consistency es la similitud media entre pares de respuestas del mismo
	// target (Jaccard sobre conjuntos de palabras), 0..1. Con <2 respuestas
	// correctas vale -1 (no medible).
	Consistency float64 `json:"consistency"`
}

// BenchReport es el resultado persistible de un benchmark.
type BenchReport struct {
	ID        string         `json:"id"`
	PromptID  string         `json:"prompt_id"`
	Reps      int            `json:"reps"`
	Summaries []BenchSummary `json:"summaries"`
	RunIDs    []string       `json:"run_ids"`
	CreatedAt time.Time      `json:"created_at"`
}

// Benchmark ejecuta reps repeticiones del prompt contra cada target y agrega
// tiempo, tokens, fallos y consistencia por target.
func (s *Service) Benchmark(ctx context.Context, promptID domain.ID, rendered string,
	targets []Target, params Params, reps int) (BenchReport, error) {
	if reps < 1 {
		reps = 1
	}
	// Expandir la matriz targets × reps y reutilizar Run (concurrencia acotada).
	expanded := make([]Target, 0, len(targets)*reps)
	for r := 0; r < reps; r++ {
		expanded = append(expanded, targets...)
	}
	results, err := s.Run(ctx, promptID, rendered, expanded, params)
	if err != nil {
		return BenchReport{}, err
	}

	report := BenchReport{
		ID: string(domain.NewID()), PromptID: string(promptID), Reps: reps,
		CreatedAt: time.Now().UTC(),
	}
	// Agrupar por target conservando el orden de la lista original.
	type key struct{ provider, model string }
	groups := map[key][]Result{}
	var order []key
	for i, r := range results {
		k := key{r.Provider, r.Model}
		if _, seen := groups[k]; !seen && i < len(targets) {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
		report.RunIDs = append(report.RunIDs, r.ID)
	}
	for _, k := range order {
		rs := groups[k]
		sum := BenchSummary{Provider: k.provider, Model: k.model, Reps: len(rs)}
		var lats []int64
		var okResponses []string
		var tin, tout int
		for _, r := range rs {
			if r.Error != "" {
				sum.Failures++
				continue
			}
			lats = append(lats, r.LatencyMS)
			tin += r.TokensIn
			tout += r.TokensOut
			okResponses = append(okResponses, r.Response)
		}
		if n := len(lats); n > 0 {
			sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
			sum.LatencyP50MS = lats[n/2]
			sum.LatencyP95MS = lats[(n*95)/100]
			sum.TokensInAvg = float64(tin) / float64(n)
			sum.TokensOutAvg = float64(tout) / float64(n)
		}
		sum.Consistency = consistency(okResponses)
		report.Summaries = append(report.Summaries, sum)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, err
	}
	// Subdirectorio propio: History() recorre solo archivos de runsDir y así
	// los informes de benchmark no se confunden con runs individuales.
	err = fsrepo.WriteFileAtomic(filepath.Join(s.runsDir(), "bench", report.ID+".json"), data)
	return report, err
}

// consistency devuelve la similitud Jaccard media entre pares de respuestas.
func consistency(responses []string) float64 {
	if len(responses) < 2 {
		return -1
	}
	sets := make([]map[string]bool, len(responses))
	for i, r := range responses {
		sets[i] = map[string]bool{}
		for _, w := range strings.Fields(strings.ToLower(r)) {
			sets[i][w] = true
		}
	}
	var total float64
	var pairs int
	for i := 0; i < len(sets); i++ {
		for j := i + 1; j < len(sets); j++ {
			total += jaccard(sets[i], sets[j])
			pairs++
		}
	}
	return total / float64(pairs)
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}
