package run

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// fakeProvider simula un proveedor con latencia y fallos controlados.
type fakeProvider struct {
	name    string
	delay   time.Duration
	fail    bool
	active  int32
	maxSeen int32
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	cur := atomic.AddInt32(&f.active, 1)
	for {
		seen := atomic.LoadInt32(&f.maxSeen)
		if cur <= seen || atomic.CompareAndSwapInt32(&f.maxSeen, seen, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.active, -1)
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return ai.Response{}, ctx.Err()
	}
	if f.fail {
		return ai.Response{}, errors.New("proveedor caído")
	}
	return ai.Response{Text: f.name + ": " + req.Prompt, TokensIn: 1, TokensOut: 2, Latency: f.delay}, nil
}

func newSvc(t *testing.T) *Service {
	ws, err := fsrepo.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewService(ws)
}

func TestRunMultiProviderPartialFailure(t *testing.T) {
	s := newSvc(t)
	ok := &fakeProvider{name: "ok"}
	bad := &fakeProvider{name: "bad", fail: true}
	results, err := s.Run(context.Background(), "p1", "hola", []Target{
		{Provider: ok, Model: "m1"}, {Provider: bad, Model: "m2"},
	}, Params{Temperature: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Error != "" || !strings.Contains(results[0].Response, "ok: hola") {
		t.Fatalf("resultado ok: %+v", results[0])
	}
	if results[1].Error == "" || results[1].Response != "" {
		t.Fatalf("el fallo debe registrarse sin abortar el resto: %+v", results[1])
	}
	if results[0].PromptHash != results[1].PromptHash {
		t.Fatal("ambos runs comparten el hash del prompt renderizado")
	}
}

func TestRunConcurrencyLimit(t *testing.T) {
	s := newSvc(t)
	s.Concurrency = 2
	p := &fakeProvider{name: "lento", delay: 50 * time.Millisecond}
	targets := make([]Target, 6)
	for i := range targets {
		targets[i] = Target{Provider: p}
	}
	start := time.Now()
	if _, err := s.Run(context.Background(), "p1", "x", targets, Params{}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&p.maxSeen); got > 2 {
		t.Fatalf("concurrencia máxima %d > límite 2", got)
	}
	// 6 tareas / 2 en paralelo * 50ms ≈ 150ms; muy por debajo de la vía secuencial (300ms).
	if elapsed := time.Since(start); elapsed > 280*time.Millisecond {
		t.Fatalf("no hubo paralelismo: %s", elapsed)
	}
}

func TestRunPersistsAndHistory(t *testing.T) {
	s := newSvc(t)
	p := &fakeProvider{name: "ok"}
	s.Run(context.Background(), "prompt-a", "uno", []Target{{Provider: p}}, Params{})
	time.Sleep(2 * time.Millisecond) // orden estable por CreatedAt
	s.Run(context.Background(), "prompt-a", "dos", []Target{{Provider: p}}, Params{})
	s.Run(context.Background(), "prompt-b", "tres", []Target{{Provider: p}}, Params{})

	hist, err := s.History("prompt-a", 0)
	if err != nil || len(hist) != 2 {
		t.Fatalf("hist=%v err=%v", hist, err)
	}
	if !strings.Contains(hist[0].Response, "dos") {
		t.Fatal("el historial debe venir con los más recientes primero")
	}
	all, _ := s.History("", 0)
	if len(all) != 3 {
		t.Fatalf("historial global: %d", len(all))
	}
	limited, _ := s.History("", 1)
	if len(limited) != 1 {
		t.Fatal("limit no respetado")
	}
}

func TestRunCancellation(t *testing.T) {
	s := newSvc(t)
	p := &fakeProvider{name: "eterno", delay: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	results, err := s.Run(ctx, "p", "x", []Target{{Provider: p}}, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("la cancelación no cortó la ejecución")
	}
	if results[0].Error == "" {
		t.Fatal("el resultado cancelado debe registrar el error")
	}
	_ = domain.ID("")
}
