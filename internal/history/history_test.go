package history

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

func setup(t *testing.T) (*fsrepo.Workspace, *Store, *domain.Prompt, string) {
	t.Helper()
	ws, err := fsrepo.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	p := &domain.Prompt{
		ID: domain.NewID(), Title: "Versionado",
		Blocks: []domain.Block{{Type: domain.BlockObjective, Enabled: true, Content: "v1"}},
		CreatedAt: now, UpdatedAt: now, Schema: 1,
	}
	rel, _, err := ws.SavePrompt(p, "")
	if err != nil {
		t.Fatal(err)
	}
	return ws, New(ws), p, rel
}

func TestSnapshotAndList(t *testing.T) {
	ws, st, p, rel := setup(t)
	v1, err := st.Snapshot(rel, "ana", "primera")
	if err != nil {
		t.Fatal(err)
	}
	if v1.Seq != 1 || v1.Author != "ana" {
		t.Fatalf("v1=%+v", v1)
	}
	// Snapshot sin cambios: deduplicado.
	v1b, err := st.Snapshot(rel, "ana", "repetida")
	if err != nil || v1b.ID != v1.ID {
		t.Fatalf("dedupe falló: %+v err=%v", v1b, err)
	}
	// Cambiar y snapshot de nuevo.
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "v2"})
	ws.SavePrompt(p, rel)
	v2, err := st.Snapshot(rel, "ana", "segunda")
	if err != nil || v2.Seq != 2 {
		t.Fatalf("v2=%+v err=%v", v2, err)
	}
	versions, err := st.List(p.ID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	if versions[0].ContentHash == versions[1].ContentHash {
		t.Fatal("hashes deben diferir")
	}
}

func TestLoadVersionContent(t *testing.T) {
	_, st, p, rel := setup(t)
	v1, _ := st.Snapshot(rel, "", "")
	loaded, raw, err := st.Load(v1.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != p.ID || !strings.Contains(string(raw), "v1") {
		t.Fatal("contenido de versión no coincide")
	}
	if _, _, err := st.Load("hash-inexistente"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("esperaba ErrNotFound: %v", err)
	}
}

func TestRestoreNeverRewritesHistory(t *testing.T) {
	ws, st, p, rel := setup(t)
	st.Snapshot(rel, "", "v1")

	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "v2"})
	ws.SavePrompt(p, rel)
	st.Snapshot(rel, "", "v2")

	// Restaurar a la versión 1 (por seq).
	restored, err := st.Restore(rel, "1")
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ws.LoadPrompt(rel)
	b, _ := e.Prompt.Block(domain.BlockObjective)
	if b.Content != "v1" {
		t.Fatalf("restore no aplicó el contenido: %q", b.Content)
	}
	versions, _ := st.List(p.ID)
	// v1, v2 y la restauración registrada como versión nueva (el snapshot
	// "antes de restaurar" se deduplica con v2 porque el disco no cambió).
	if len(versions) != 3 {
		t.Fatalf("la restauración debe añadir versiones, no borrarlas: %d", len(versions))
	}
	if restored.Seq != 3 {
		t.Fatalf("la restauración es la versión más nueva: %+v", restored)
	}
	if !strings.Contains(versions[2].Comment, "restaurado") {
		t.Fatalf("comentario de restauración: %q", versions[2].Comment)
	}
}

func TestRestoreUnknownVersion(t *testing.T) {
	_, st, _, rel := setup(t)
	st.Snapshot(rel, "", "")
	if _, err := st.Restore(rel, "99"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("esperaba ErrNotFound: %v", err)
	}
}

func TestObjectsAreDeduplicatedByHash(t *testing.T) {
	ws, st, p, rel := setup(t)
	st.Snapshot(rel, "", "")
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "v2"})
	ws.SavePrompt(p, rel)
	st.Snapshot(rel, "", "")
	// Volver exactamente al contenido v1 → mismo hash, sin objeto nuevo.
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "v1"})
	p.UpdatedAt = p.CreatedAt
	ws.SavePrompt(p, rel)
	st.Snapshot(rel, "", "vuelta a v1")

	objs, _ := os.ReadDir(filepath.Join(ws.Root, fsrepo.DirMeta, "history", "objects"))
	if len(objs) != 2 {
		t.Fatalf("objetos=%d, la deduplicación content-addressed falló", len(objs))
	}
	versions, _ := st.List(p.ID)
	if len(versions) != 3 {
		t.Fatalf("versiones=%d", len(versions))
	}
}

func TestListToleratesCorruptLogLine(t *testing.T) {
	ws, st, p, rel := setup(t)
	st.Snapshot(rel, "", "")
	logPath := filepath.Join(ws.Root, fsrepo.DirMeta, "history", string(p.ID)+".jsonl")
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("{linea rota\n")
	f.Close()
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "v2"})
	ws.SavePrompt(p, rel)
	if _, err := st.Snapshot(rel, "", ""); err != nil {
		t.Fatal(err)
	}
	versions, err := st.List(p.ID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("log corrupto debe tolerarse: %d err=%v", len(versions), err)
	}
}
