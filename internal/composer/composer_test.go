package composer

import (
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func layers() map[string]*domain.Prompt {
	return map[string]*domain.Prompt{
		"base": {ID: "base", Title: "Base", Variables: map[string]string{"tone": "formal"},
			Tags: []string{"base"},
			Blocks: []domain.Block{
				{Type: domain.BlockRole, Enabled: true, Content: "Eres ingeniero."},
				{Type: domain.BlockRules, Enabled: true, Content: "Regla base."},
				{Type: domain.BlockNotes, Enabled: false, Content: "oculta"},
			}},
		"godot": {ID: "godot", Title: "Godot", Variables: map[string]string{"tone": "directo"},
			Tags: []string{"godot"},
			Blocks: []domain.Block{
				{Type: domain.BlockRules, Enabled: true, Content: "Usa GDScript."},
				{Type: domain.BlockOutput, Enabled: true, Content: "Escenas .tscn."},
			}},
	}
}

func resolver(t *testing.T) Resolver {
	m := layers()
	return func(ref string) (*domain.Prompt, error) {
		if p, ok := m[ref]; ok {
			return p, nil
		}
		return nil, domain.ErrNotFound
	}
}

func TestComposeConcat(t *testing.T) {
	c := &Composition{Title: "Final", Strategy: StrategyConcat, Layers: []Layer{
		{Ref: "base", Enabled: true}, {Ref: "godot", Enabled: true},
	}}
	out, err := Compose(c, resolver(t))
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := out.Block(domain.BlockRules)
	if !strings.Contains(rules.Content, "Regla base.") || !strings.Contains(rules.Content, "Usa GDScript.") {
		t.Fatalf("concat de rules: %q", rules.Content)
	}
	if idx := strings.Index(rules.Content, "Regla base."); idx > strings.Index(rules.Content, "Usa GDScript.") {
		t.Fatal("el orden de capas debe conservarse (base primero)")
	}
	if _, ok := out.Block(domain.BlockNotes); ok {
		t.Fatal("bloques desactivados de una capa no deben entrar")
	}
	if out.Variables["tone"] != "directo" {
		t.Fatal("la capa superior debe ganar en variables")
	}
	if len(out.Tags) != 2 {
		t.Fatalf("tags fusionados: %v", out.Tags)
	}
}

func TestComposeReplace(t *testing.T) {
	c := &Composition{Title: "Final", Strategy: StrategyReplace, Layers: []Layer{
		{Ref: "base", Enabled: true}, {Ref: "godot", Enabled: true},
	}}
	out, err := Compose(c, resolver(t))
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := out.Block(domain.BlockRules)
	if rules.Content != "Usa GDScript." {
		t.Fatalf("replace: %q", rules.Content)
	}
	role, _ := out.Block(domain.BlockRole)
	if role.Content != "Eres ingeniero." {
		t.Fatal("bloques no redefinidos se conservan")
	}
}

func TestComposeDisabledLayerSkipped(t *testing.T) {
	c := &Composition{Title: "Final", Strategy: StrategyConcat, Layers: []Layer{
		{Ref: "base", Enabled: true}, {Ref: "godot", Enabled: false},
	}}
	out, _ := Compose(c, resolver(t))
	if _, ok := out.Block(domain.BlockOutput); ok {
		t.Fatal("capa desactivada no debe aportar bloques")
	}
}

func TestComposeErrors(t *testing.T) {
	if _, err := Compose(&Composition{Title: "x", Layers: []Layer{{Ref: "nope", Enabled: true}}}, resolver(t)); err == nil {
		t.Fatal("capa inexistente debe fallar")
	}
	if _, err := Compose(&Composition{Title: "x", Layers: []Layer{{Ref: "base", Enabled: false}}}, resolver(t)); err == nil {
		t.Fatal("sin capas activas debe fallar")
	}
}

func TestParseComposition(t *testing.T) {
	good := []byte("title: Final\nstrategy: concat\nlayers:\n  - {ref: base, enabled: true}\n")
	c, err := ParseComposition(good)
	if err != nil || c.Title != "Final" || c.Strategy != StrategyConcat {
		t.Fatalf("c=%+v err=%v", c, err)
	}
	// Estrategia por defecto.
	c, _ = ParseComposition([]byte("title: X\nlayers:\n  - {ref: a, enabled: true}\n"))
	if c.Strategy != StrategyConcat {
		t.Fatal("estrategia por defecto debe ser concat")
	}
	for _, bad := range []string{"", "title: solo", "title: x\nstrategy: magia\nlayers: [{ref: a, enabled: true}]", "{{{"} {
		if _, err := ParseComposition([]byte(bad)); err == nil {
			t.Errorf("%q debería fallar", bad)
		}
	}
}
