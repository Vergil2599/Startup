package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/ai"
)

// variableProvider responde con variantes para medir consistencia.
type variableProvider struct {
	name      string
	responses []string
	i         int
	fail      bool
}

func (v *variableProvider) Name() string { return v.name }
func (v *variableProvider) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	if v.fail {
		return ai.Response{}, context.DeadlineExceeded
	}
	r := v.responses[v.i%len(v.responses)]
	v.i++
	return ai.Response{Text: r, TokensIn: 5, TokensOut: len(strings.Fields(r)), Latency: 10 * time.Millisecond}, nil
}

func TestBenchmarkAggregates(t *testing.T) {
	s := newSvc(t)
	s.Concurrency = 1 // respuestas deterministas por orden
	stable := &variableProvider{name: "estable", responses: []string{"la respuesta es siempre igual"}}
	chaotic := &variableProvider{name: "caotico", responses: []string{
		"lunes martes miercoles", "rojo verde azul", "uno dos tres cuatro",
	}}
	report, err := s.Benchmark(context.Background(), "p1", "pregunta",
		[]Target{{Provider: stable, Model: "m1"}, {Provider: chaotic, Model: "m2"}},
		Params{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Summaries) != 2 || len(report.RunIDs) != 6 {
		t.Fatalf("report=%+v", report)
	}
	var st, ch BenchSummary
	for _, sum := range report.Summaries {
		if sum.Provider == "estable" {
			st = sum
		} else {
			ch = sum
		}
	}
	if st.Consistency != 1.0 {
		t.Fatalf("proveedor estable debe tener consistencia 1.0: %v", st.Consistency)
	}
	if ch.Consistency >= 0.5 {
		t.Fatalf("proveedor caótico debe tener consistencia baja: %v", ch.Consistency)
	}
	if st.Reps != 3 || st.Failures != 0 || st.TokensOutAvg <= 0 {
		t.Fatalf("agregados: %+v", st)
	}
	if st.LatencyP50MS <= 0 || st.LatencyP95MS < st.LatencyP50MS {
		t.Fatalf("latencias: %+v", st)
	}
}

func TestBenchmarkCountsFailures(t *testing.T) {
	s := newSvc(t)
	bad := &variableProvider{name: "roto", fail: true}
	report, err := s.Benchmark(context.Background(), "p1", "x",
		[]Target{{Provider: bad}}, Params{}, 4)
	if err != nil {
		t.Fatal(err)
	}
	sum := report.Summaries[0]
	if sum.Failures != 4 || sum.Consistency != -1 {
		t.Fatalf("fallos: %+v", sum)
	}
}

func TestBenchmarkPersistsReport(t *testing.T) {
	s := newSvc(t)
	p := &variableProvider{name: "ok", responses: []string{"hola"}}
	report, _ := s.Benchmark(context.Background(), "p1", "x", []Target{{Provider: p}}, Params{}, 2)
	path := filepath.Join(s.ws.Root, ".pes", "runs", "bench", report.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"consistency"`) {
		t.Fatalf("informe no persistido: %v", err)
	}
	// Los runs individuales también quedan en el historial.
	hist, _ := s.History("p1", 0)
	if len(hist) != 2 {
		t.Fatalf("runs individuales: %d", len(hist))
	}
}

func TestJaccard(t *testing.T) {
	set := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, w := range strings.Fields(s) {
			m[w] = true
		}
		return m
	}
	if j := jaccard(set("a b c"), set("a b c")); j != 1 {
		t.Fatalf("idénticos: %v", j)
	}
	if j := jaccard(set("a b"), set("c d")); j != 0 {
		t.Fatalf("disjuntos: %v", j)
	}
	if j := jaccard(set(""), set("")); j != 1 {
		t.Fatalf("vacíos: %v", j)
	}
}
