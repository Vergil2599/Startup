package sqlindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// TestPerf10kBudget verifica los presupuestos de rendimiento del M0 con un
// workspace sintético de 10 000 prompts: indexación completa < 30 s y
// búsqueda FTS p95 < 50 ms. Se omite con `go test -short`.
func TestPerf10kBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("presupuesto de rendimiento: omitido en -short")
	}
	const n = 10000
	ws, err := fsrepo.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Generar el corpus escribiendo archivos directamente (serialización real).
	topics := []string{"godot", "sql", "devops", "aws", "writing", "security", "react", "kubernetes"}
	genStart := time.Now()
	for i := 0; i < n; i++ {
		topic := topics[i%len(topics)]
		p := &domain.Prompt{
			ID:    domain.NewID(),
			Title: fmt.Sprintf("Prompt %s %05d", topic, i),
			Tags:  []string{topic},
			Blocks: []domain.Block{
				{Type: domain.BlockRole, Enabled: true, Content: fmt.Sprintf("Experto en %s nivel %d.", topic, i%10)},
				{Type: domain.BlockObjective, Enabled: true, Content: fmt.Sprintf("Resolver la tarea %05d de %s con precisión.", i, topic)},
				{Type: domain.BlockOutput, Enabled: true, Content: "Markdown estructurado."},
			},
			Schema: 1,
		}
		data, serr := fsrepo.Serialize(p)
		if serr != nil {
			t.Fatal(serr)
		}
		rel := filepath.Join(ws.Root, fsrepo.DirPrompts, fmt.Sprintf("p-%05d.md", i))
		if werr := os.WriteFile(rel, data, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	t.Logf("generación de %d archivos: %s", n, time.Since(genStart))

	ix, err := Open(filepath.Join(ws.Root, fsrepo.DirMeta, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()

	idxStart := time.Now()
	stats, err := ix.Sync(ws)
	idxDur := time.Since(idxStart)
	if err != nil || stats.Indexed != n {
		t.Fatalf("sync: %+v err=%v", stats, err)
	}
	t.Logf("indexación completa de %d prompts: %s", n, idxDur)
	if idxDur > 30*time.Second {
		t.Fatalf("PRESUPUESTO ROTO: indexación %s > 30s", idxDur)
	}

	// p95 de 100 búsquedas variadas.
	queries := []string{"godot", "precisión", "kubernetes nivel", "tarea 04", "writing", "markdown"}
	var durations []time.Duration
	for i := 0; i < 100; i++ {
		q := queries[i%len(queries)]
		st := time.Now()
		hits, serr := ix.Search(q, 50)
		durations = append(durations, time.Since(st))
		if serr != nil {
			t.Fatal(serr)
		}
		if len(hits) == 0 {
			t.Fatalf("la consulta %q no devolvió resultados", q)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[94]
	t.Logf("búsqueda p50=%s p95=%s", durations[49], p95)
	if p95 > 50*time.Millisecond {
		t.Fatalf("PRESUPUESTO ROTO: búsqueda p95 %s > 50ms", p95)
	}

	// Reindexado incremental de un solo archivo tras el corpus completo.
	one := filepath.Join(fsrepo.DirPrompts, "p-00000.md")
	e, _ := ws.LoadPrompt(one)
	e.Prompt.SetBlock(domain.Block{Type: domain.BlockNotes, Enabled: true, Content: "cambio incremental"})
	ws.SavePrompt(e.Prompt, one)
	incStart := time.Now()
	stats, err = ix.Sync(ws)
	incDur := time.Since(incStart)
	if err != nil || stats.Indexed != 1 || stats.Skipped != n-1 {
		t.Fatalf("incremental: %+v err=%v", stats, err)
	}
	t.Logf("sync incremental (1 cambio sobre %d): %s", n, incDur)
}
