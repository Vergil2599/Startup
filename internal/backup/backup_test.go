package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

func seedWS(t *testing.T) *fsrepo.Workspace {
	t.Helper()
	ws, err := fsrepo.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := &domain.Prompt{ID: domain.NewID(), Title: "Respaldable",
		Blocks: []domain.Block{{Type: domain.BlockObjective, Enabled: true, Content: "contenido á é"}},
		Schema: 1}
	if _, _, err := ws.SavePrompt(p, ""); err != nil {
		t.Fatal(err)
	}
	ws.SaveVariables("global", map[string]string{"k": "v"})
	return ws
}

func TestCreateAndEntries(t *testing.T) {
	ws := seedWS(t)
	path, err := Create(ws, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".tar.gz") {
		t.Fatalf("nombre: %s", path)
	}
	names, err := Entries(path, "")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(names, "\n")
	if !strings.Contains(joined, "prompts/respaldable.md") ||
		!strings.Contains(joined, "variables/global.yaml") {
		t.Fatalf("contenido del backup:\n%s", joined)
	}
	// El índice (derivado) jamás entra en el backup.
	if strings.Contains(joined, "index.db") {
		t.Fatal("el índice desechable no debe respaldarse")
	}
}

func TestEncryptedRoundTripAndWrongPassphrase(t *testing.T) {
	ws := seedWS(t)
	path, err := Create(ws, "secreta123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".age") {
		t.Fatalf("backup cifrado sin extensión .age: %s", path)
	}
	names, err := Entries(path, "secreta123")
	if err != nil || len(names) == 0 {
		t.Fatalf("descifrado falló: %v", err)
	}
	if _, err := Entries(path, "incorrecta"); err == nil {
		t.Fatal("passphrase incorrecta debe fallar")
	}
	if _, err := Entries(path, ""); err == nil {
		t.Fatal("backup cifrado sin passphrase debe fallar con mensaje claro")
	}
	// El archivo cifrado no contiene texto plano.
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "Respaldable") || strings.Contains(string(raw), "contenido") {
		t.Fatal("el backup cifrado filtra texto plano")
	}
}

func TestRotate(t *testing.T) {
	ws := seedWS(t)
	// Crear 4 backups con nombres distintos (timestamps forzados).
	for i := 0; i < 4; i++ {
		path, err := Create(ws, "")
		if err != nil {
			t.Fatal(err)
		}
		// Renombrar para garantizar orden temporal distinto.
		newName := filepath.Join(filepath.Dir(path),
			"pes-backup-2026010"+string(rune('1'+i))+"-000000.tar.gz")
		os.Rename(path, newName)
	}
	removed, err := Rotate(ws, 2)
	if err != nil || removed != 2 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	names, _ := List(ws)
	if len(names) != 2 {
		t.Fatalf("quedan %d backups", len(names))
	}
	// Se conservan los más recientes.
	if !strings.Contains(names[0], "20260104") || !strings.Contains(names[1], "20260103") {
		t.Fatalf("rotación conservó los equivocados: %v", names)
	}
}

func TestListEmptyWorkspace(t *testing.T) {
	ws := seedWS(t)
	names, err := List(ws)
	if err != nil || names != nil {
		t.Fatalf("names=%v err=%v", names, err)
	}
	if _, err := Rotate(ws, 3); err != nil {
		t.Fatal("rotar sin backups no es error")
	}
	_ = time.Now()
}
