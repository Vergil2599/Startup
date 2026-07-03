package export

import (
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func TestRegistryHasBuiltinsAndRejectsDuplicates(t *testing.T) {
	want := []string{"html", "json", "md", "pdf", "txt", "yaml"}
	got := Formats()
	if len(got) != len(want) {
		t.Fatalf("Formats()=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Formats()=%v, quería %v", got, want)
		}
	}
	if err := Register(mdCodec{}); err == nil {
		t.Fatal("duplicado debe rechazarse")
	}
	if _, err := Get("docx"); err == nil {
		t.Fatal("formato no registrado debe fallar")
	}
}

func TestTxtStrictIsNotEnforced(t *testing.T) {
	// txt exporta en modo no estricto: variables sin resolver quedan visibles.
	p := &domain.Prompt{ID: "x", Title: "T", Blocks: []domain.Block{
		{Type: domain.BlockObjective, Enabled: true, Content: "usa {{missing}}"},
	}}
	c, _ := Get("txt")
	data, err := c.Export(Input{Prompt: p})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "usa {{missing}}\n" {
		t.Fatalf("txt=%q", data)
	}
}

func TestFromStructuredRoundTrip(t *testing.T) {
	p := &domain.Prompt{ID: "id1", Title: "T", Tags: []string{"a"},
		Blocks: []domain.Block{{Type: domain.BlockRole, Enabled: false, Content: "c"}}}
	got := FromStructured(toStructured(p))
	if got.Title != p.Title || len(got.Blocks) != 1 || got.Blocks[0].Enabled {
		t.Fatalf("round-trip estructurado: %+v", got)
	}
}

func TestHTMLEscapesAndRenders(t *testing.T) {
	p := &domain.Prompt{ID: "x", Title: "Con <script>", Blocks: []domain.Block{
		{Type: domain.BlockObjective, Enabled: true, Content: "usa {{lang}} & <b>html</b>"},
	}}
	c, _ := Get("html")
	data, err := c.Export(Input{Prompt: p, Vars: domain.VariableSet{Global: map[string]string{"lang": "Go"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "<script>") || !strings.Contains(s, "&lt;script&gt;") {
		t.Fatal("el título debe escaparse")
	}
	if !strings.Contains(s, "usa Go &amp;") {
		t.Fatalf("variables resueltas y contenido escapado: %s", s)
	}
}

func TestPDFProducesValidHeader(t *testing.T) {
	p := &domain.Prompt{ID: "x", Title: "PDF con acentos áéí", Blocks: []domain.Block{
		{Type: domain.BlockObjective, Enabled: true, Content: "Objetivo ñoño de prueba."},
	}}
	c, _ := Get("pdf")
	data, err := c.Export(Input{Prompt: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 500 || string(data[:5]) != "%PDF-" {
		t.Fatalf("no parece un PDF: %d bytes, cabecera %q", len(data), data[:8])
	}
}
