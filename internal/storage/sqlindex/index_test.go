package sqlindex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

func newWS(t *testing.T) *fsrepo.Workspace {
	t.Helper()
	ws, err := fsrepo.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func newIndex(t *testing.T) *Index {
	t.Helper()
	ix, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ix.Close() })
	return ix
}

func save(t *testing.T, ws *fsrepo.Workspace, title, content string, tags ...string) *domain.Prompt {
	t.Helper()
	p := &domain.Prompt{
		ID: domain.NewID(), Title: title, Tags: domain.NormalizeTags(tags),
		Category: "software-development",
		Blocks: []domain.Block{
			{Type: domain.BlockObjective, Enabled: true, Content: content},
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Schema: 1,
	}
	if _, _, err := ws.SavePrompt(p, ""); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSyncAndSearch(t *testing.T) {
	ws, ix := newWS(t), newIndex(t)
	save(t, ws, "Revisor Go", "Detectar defectos de concurrencia en goroutines", "go", "review")
	save(t, ws, "Godot GDScript", "Generar escenas de plataformas", "godot")

	stats, err := ix.Sync(ws)
	if err != nil || stats.Indexed != 2 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	hits, err := ix.Search("goroutines", 10)
	if err != nil || len(hits) != 1 || hits[0].Title != "Revisor Go" {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
	// Búsqueda por prefijo.
	hits, _ = ix.Search("gorout", 10)
	if len(hits) != 1 {
		t.Fatalf("búsqueda por prefijo falló: %+v", hits)
	}
	// Sin resultados no es error.
	hits, err = ix.Search("inexistente_xyz", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
	// Entrada hostil no rompe el FTS.
	if _, err := ix.Search(`"malicioso OR ) NEAR(`, 10); err != nil {
		t.Fatalf("entrada hostil causó error: %v", err)
	}
}

func TestSyncIncrementalSkipsUnchanged(t *testing.T) {
	ws, ix := newWS(t), newIndex(t)
	p := save(t, ws, "Uno", "contenido original", "a")
	if _, err := ix.Sync(ws); err != nil {
		t.Fatal(err)
	}
	stats, _ := ix.Sync(ws)
	if stats.Skipped != 1 || stats.Indexed != 0 {
		t.Fatalf("segunda pasada debe saltar sin cambios: %+v", stats)
	}
	// Modificar → reindexa solo ese.
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "contenido nuevo unico"})
	p.UpdatedAt = time.Now().UTC()
	if _, _, err := ws.SavePrompt(p, filepath.Join(fsrepo.DirPrompts, "uno.md")); err != nil {
		t.Fatal(err)
	}
	stats, _ = ix.Sync(ws)
	if stats.Indexed != 1 {
		t.Fatalf("cambio no detectado: %+v", stats)
	}
	if hits, _ := ix.Search("unico", 10); len(hits) != 1 {
		t.Fatal("el contenido nuevo no es buscable")
	}
	if hits, _ := ix.Search("original", 10); len(hits) != 0 {
		t.Fatal("el contenido viejo sigue indexado")
	}
}

func TestSyncRemovesDeleted(t *testing.T) {
	ws, ix := newWS(t), newIndex(t)
	save(t, ws, "Efímero", "desaparecerá")
	ix.Sync(ws)
	os.Remove(filepath.Join(ws.Root, fsrepo.DirPrompts, "efimero.md"))
	stats, _ := ix.Sync(ws)
	if stats.Removed != 1 {
		t.Fatalf("borrado externo no purgado: %+v", stats)
	}
	if n, _ := ix.Count(); n != 0 {
		t.Fatalf("Count=%d", n)
	}
}

func TestListFilters(t *testing.T) {
	ws, ix := newWS(t), newIndex(t)
	fav := save(t, ws, "Favorito", "x", "go")
	fav.Favorite = true
	ws.SavePrompt(fav, filepath.Join(fsrepo.DirPrompts, "favorito.md"))
	save(t, ws, "Otro", "y", "sql")
	ix.Sync(ws)

	if hits, _ := ix.List(ListFilter{Tag: "go"}); len(hits) != 1 || hits[0].Title != "Favorito" {
		t.Fatalf("filtro tag: %+v", hits)
	}
	if hits, _ := ix.List(ListFilter{Favorites: true}); len(hits) != 1 {
		t.Fatalf("filtro favoritos: %+v", hits)
	}
	if hits, _ := ix.List(ListFilter{Category: "software-development"}); len(hits) != 2 {
		t.Fatalf("filtro categoría: %+v", hits)
	}
	if hits, _ := ix.List(ListFilter{}); len(hits) != 2 {
		t.Fatalf("sin filtro: %+v", hits)
	}
	// Tags cargados en los hits.
	hits, _ := ix.List(ListFilter{Tag: "go"})
	if len(hits[0].Tags) != 1 || hits[0].Tags[0] != "go" {
		t.Fatalf("tags del hit: %+v", hits[0].Tags)
	}
}

func TestSetScore(t *testing.T) {
	ws, ix := newWS(t), newIndex(t)
	p := save(t, ws, "Puntuado", "x")
	ix.Sync(ws)
	if err := ix.SetScore(p.ID, 85); err != nil {
		t.Fatal(err)
	}
	hits, _ := ix.List(ListFilter{})
	if hits[0].Score == nil || *hits[0].Score != 85 {
		t.Fatalf("score no persistido: %+v", hits[0].Score)
	}
}

func TestRebuildEqualsSync(t *testing.T) {
	ws, ix := newWS(t), newIndex(t)
	for i := 0; i < 5; i++ {
		save(t, ws, fmt.Sprintf("Prompt %d", i), fmt.Sprintf("contenido %d", i))
	}
	ix.Sync(ws)
	stats, err := ix.Rebuild(ws)
	if err != nil || stats.Indexed != 5 {
		t.Fatalf("rebuild: %+v err=%v", stats, err)
	}
	if n, _ := ix.Count(); n != 5 {
		t.Fatalf("Count tras rebuild=%d", n)
	}
}

func TestOpenOnDiskAndReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "index.db")
	ix, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ws := newWS(t)
	save(t, ws, "Persistente", "en disco")
	ix.Sync(ws)
	ix.Close()

	ix2, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ix2.Close()
	if n, _ := ix2.Count(); n != 1 {
		t.Fatalf("el índice no sobrevivió al reopen: %d", n)
	}
}
