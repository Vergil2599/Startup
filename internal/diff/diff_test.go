package diff

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func TestStructural(t *testing.T) {
	a := &domain.Prompt{Title: "A", Blocks: []domain.Block{
		{Type: domain.BlockRole, Enabled: true, Content: "rol"},
		{Type: domain.BlockRules, Enabled: true, Content: "reglas"},
		{Type: domain.BlockNotes, Enabled: true, Content: "nota"},
	}}
	b := &domain.Prompt{Title: "B", Blocks: []domain.Block{
		{Type: domain.BlockRole, Enabled: false, Content: "rol"},        // toggled
		{Type: domain.BlockRules, Enabled: true, Content: "cambiadas"},  // modified
		{Type: domain.BlockOutput, Enabled: true, Content: "salida"},    // added
	}} // notes: removed
	d := Structural(a, b)
	kinds := map[domain.BlockType]domain.DiffKind{}
	for _, bd := range d.Blocks {
		kinds[bd.Type] = bd.Kind
	}
	if kinds[domain.BlockRole] != domain.DiffToggled ||
		kinds[domain.BlockRules] != domain.DiffModified ||
		kinds[domain.BlockOutput] != domain.DiffAdded ||
		kinds[domain.BlockNotes] != domain.DiffRemoved {
		t.Fatalf("kinds=%v", kinds)
	}
	if !d.TitleDiff || d.Equal {
		t.Fatal("TitleDiff/Equal incorrectos")
	}
	if d.WordsA != 3 || d.WordsB != 3 {
		t.Fatalf("palabras: %d/%d", d.WordsA, d.WordsB)
	}
}

func TestStructuralEqual(t *testing.T) {
	p := &domain.Prompt{Title: "X", Blocks: []domain.Block{
		{Type: domain.BlockRole, Enabled: true, Content: "igual"},
	}}
	q := &domain.Prompt{Title: "X", Blocks: []domain.Block{
		{Type: domain.BlockRole, Enabled: true, Content: "igual"},
	}}
	if d := Structural(p, q); !d.Equal || len(d.Blocks) != 0 {
		t.Fatalf("prompts idénticos: %+v", d)
	}
}

func TestLinesBasic(t *testing.T) {
	ops := Lines("a\nb\nc", "a\nX\nc")
	want := []Op{{"eq", "a"}, {"del", "b"}, {"add", "X"}, {"eq", "c"}}
	if len(ops) != len(want) {
		t.Fatalf("ops=%v", ops)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("ops[%d]=%v, quería %v", i, ops[i], want[i])
		}
	}
}

func TestLinesEmptySides(t *testing.T) {
	if ops := Lines("", "a\nb"); len(ops) != 2 || ops[0].Kind != "add" {
		t.Fatalf("vacío→texto: %v", ops)
	}
	if ops := Lines("a\nb", ""); len(ops) != 2 || ops[0].Kind != "del" {
		t.Fatalf("texto→vacío: %v", ops)
	}
	if ops := Lines("", ""); len(ops) != 0 {
		t.Fatalf("vacío→vacío: %v", ops)
	}
}

// Propiedad: aplicar las operaciones del diff sobre A reconstruye B.
func TestLinesPatchProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	words := []string{"uno", "dos", "tres", "cuatro", "cinco"}
	randText := func() string {
		n := rng.Intn(8)
		lines := make([]string, n)
		for i := range lines {
			lines[i] = words[rng.Intn(len(words))]
		}
		return strings.Join(lines, "\n")
	}
	for i := 0; i < 200; i++ {
		a, b := randText(), randText()
		var rebuilt []string
		for _, op := range Lines(a, b) {
			if op.Kind == "eq" || op.Kind == "add" {
				rebuilt = append(rebuilt, op.Line)
			}
		}
		got := strings.Join(rebuilt, "\n")
		wantB := strings.TrimRight(b, "\n")
		if got != wantB {
			t.Fatalf("caso %d: patch(A,diff) != B\nA=%q\nB=%q\ngot=%q", i, a, b, got)
		}
	}
}

func TestUnifiedFormat(t *testing.T) {
	out := Unified("a", "b")
	if !strings.Contains(out, "- a") || !strings.Contains(out, "+ b") {
		t.Fatalf("unified=%q", out)
	}
}
