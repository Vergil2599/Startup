package template

import (
	"errors"
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func vars(prompt, project, global map[string]string) domain.VariableSet {
	return domain.VariableSet{Prompt: prompt, Project: project, Global: global}
}

func TestRenderStringBasics(t *testing.T) {
	out, missing := RenderString("Usa {{language}} con {{framework}}.",
		vars(nil, map[string]string{"language": "Go"}, map[string]string{"framework": "Wails"}))
	if out != "Usa Go con Wails." || len(missing) != 0 {
		t.Fatalf("out=%q missing=%v", out, missing)
	}
}

func TestRenderStringMissingKeepsPlaceholder(t *testing.T) {
	out, missing := RenderString("Hola {{who}}", vars(nil, nil, nil))
	if out != "Hola {{who}}" {
		t.Fatalf("placeholder debe conservarse: %q", out)
	}
	if len(missing) != 1 || missing[0] != "who" {
		t.Fatalf("missing=%v", missing)
	}
}

func TestRenderStringNoInjection(t *testing.T) {
	// El valor de una variable contiene sintaxis de plantilla: NO debe expandirse.
	out, missing := RenderString("{{a}}", vars(map[string]string{"a": "{{b}}", "b": "x"}, nil, nil))
	if out != "{{b}}" || len(missing) != 0 {
		t.Fatalf("inyección detectada: out=%q missing=%v", out, missing)
	}
}

func TestRenderStringFilters(t *testing.T) {
	cases := []struct{ in, want string }{
		{"{{name|upper}}", "MI APP"},
		{"{{name|kebab}}", "mi-app"},
		{"{{name|snake}}", "mi_app"},
		{"{{name|title}}", "Mi App"},
	}
	v := vars(map[string]string{"name": "mi app"}, nil, nil)
	for _, c := range cases {
		out, missing := RenderString(c.in, v)
		if out != c.want || len(missing) != 0 {
			t.Errorf("%s => %q (missing %v), quería %q", c.in, out, missing, c.want)
		}
	}
}

func TestRenderStringUnknownFilter(t *testing.T) {
	_, missing := RenderString("{{name|nope}}", vars(map[string]string{"name": "x"}, nil, nil))
	if len(missing) != 1 {
		t.Fatalf("filtro desconocido debe reportarse: %v", missing)
	}
}

func TestRegisterFilterRejectsDuplicates(t *testing.T) {
	if err := RegisterFilter("upper", strings.ToUpper); err == nil {
		t.Fatal("no debe permitir redefinir filtros builtin")
	}
	if err := RegisterFilter("reverse_test", func(s string) string { return s }); err != nil {
		t.Fatalf("registro de filtro nuevo falló: %v", err)
	}
}

func samplePrompt() *domain.Prompt {
	return &domain.Prompt{
		ID: domain.NewID(), Title: "Test",
		Variables: map[string]string{"lang": "Go"},
		Blocks: []domain.Block{
			{Type: domain.BlockRole, Enabled: true, Content: "Eres experto en {{lang}}."},
			{Type: domain.BlockRules, Enabled: false, Content: "Ignorado {{nope}}"},
			{Type: domain.BlockOutput, Enabled: true, Content: "Responde en {{format}}."},
			{Type: domain.BlockNotes, Enabled: true, Content: "   "}, // vacío: se omite
		},
	}
}

func TestRenderPrompt(t *testing.T) {
	p := samplePrompt()
	out, err := RenderPrompt(p, vars(nil, map[string]string{"format": "JSON"}, nil), RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	want := "Eres experto en Go.\n\nResponde en JSON."
	if out != want {
		t.Fatalf("out=%q", out)
	}
}

func TestRenderPromptHeadings(t *testing.T) {
	p := samplePrompt()
	out, _ := RenderPrompt(p, vars(nil, map[string]string{"format": "JSON"}, nil), RenderOpts{IncludeHeadings: true})
	if !strings.Contains(out, "## Role\n") || !strings.Contains(out, "## Output\n") {
		t.Fatalf("faltan encabezados: %q", out)
	}
}

func TestRenderPromptStrictFailsOnMissing(t *testing.T) {
	p := samplePrompt()
	_, err := RenderPrompt(p, vars(nil, nil, nil), RenderOpts{Strict: true})
	if !errors.Is(err, domain.ErrUnresolvedVars) {
		t.Fatalf("esperaba ErrUnresolvedVars, obtuve %v", err)
	}
	// El bloque desactivado con {{nope}} no debe contar como sin resolver.
	if err != nil && strings.Contains(err.Error(), "nope") {
		t.Fatal("variables de bloques desactivados no deben validarse")
	}
}

func TestExtractAndUnresolved(t *testing.T) {
	p := samplePrompt()
	all := ExtractVariables(p)
	if len(all) != 3 { // lang, nope, format (incluye desactivados)
		t.Fatalf("ExtractVariables=%v", all)
	}
	missing := UnresolvedVariables(p, vars(nil, nil, nil))
	if len(missing) != 1 || missing[0] != "format" {
		t.Fatalf("UnresolvedVariables=%v (lang resuelve local, nope está desactivado)", missing)
	}
}

func TestInheritanceMergeAndOverride(t *testing.T) {
	base := &domain.Prompt{ID: "base", Title: "Base", Category: "software-development",
		Variables: map[string]string{"tone": "formal", "lang": "Go"},
		Blocks: []domain.Block{
			{Type: domain.BlockRole, Enabled: true, Content: "rol base"},
			{Type: domain.BlockRules, Enabled: true, Content: "reglas base"},
		}}
	child := &domain.Prompt{ID: "child", Title: "Child", Extends: "base",
		Variables: map[string]string{"lang": "Python"},
		Blocks: []domain.Block{
			{Type: domain.BlockRules, Enabled: true, Content: "reglas hijas"},
			{Type: domain.BlockOutput, Enabled: true, Content: "salida"},
		}}
	lookup := func(id domain.ID) (*domain.Prompt, error) {
		if id == "base" {
			return base, nil
		}
		return nil, domain.ErrNotFound
	}
	got, err := ResolveInheritance(child, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := got.Block(domain.BlockRules); b.Content != "reglas hijas" {
		t.Fatalf("override falló: %q", b.Content)
	}
	if b, ok := got.Block(domain.BlockRole); !ok || b.Content != "rol base" {
		t.Fatal("herencia del bloque del padre falló")
	}
	if got.Variables["lang"] != "Python" || got.Variables["tone"] != "formal" {
		t.Fatalf("merge de variables falló: %v", got.Variables)
	}
	if got.Category != "software-development" {
		t.Fatalf("categoría heredada falló: %q", got.Category)
	}
	// Orden: bloques del padre primero.
	if got.Blocks[0].Type != domain.BlockRole {
		t.Fatalf("orden de herencia incorrecto: %v", got.Blocks[0].Type)
	}
}

func TestInheritanceCycleDetection(t *testing.T) {
	a := &domain.Prompt{ID: "a", Title: "A", Extends: "b"}
	b := &domain.Prompt{ID: "b", Title: "B", Extends: "a"}
	lookup := func(id domain.ID) (*domain.Prompt, error) {
		if id == "a" {
			return a, nil
		}
		return b, nil
	}
	if _, err := ResolveInheritance(a, lookup); !errors.Is(err, domain.ErrTemplateCycle) {
		t.Fatalf("esperaba ErrTemplateCycle, obtuve %v", err)
	}
}

// Propiedad: renderizar dos veces con las mismas variables es idempotente en
// resultado (determinismo), y un texto sin placeholders queda intacto.
func TestRenderDeterministicAndIdentity(t *testing.T) {
	v := vars(map[string]string{"x": "1"}, nil, nil)
	texts := []string{"sin variables", "{{x}} y {{x}}", "llaves { sueltas }", "{{ x }}"}
	for _, s := range texts {
		a, _ := RenderString(s, v)
		b, _ := RenderString(s, v)
		if a != b {
			t.Fatalf("render no determinista para %q", s)
		}
	}
	if out, _ := RenderString("llaves { sueltas } {no-var}", v); out != "llaves { sueltas } {no-var}" {
		t.Fatalf("texto sin placeholders alterado: %q", out)
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("vacío = 0")
	}
	// ~4 chars/token: 400 runas ≈ 100 tokens (±).
	long := strings.Repeat("abcd", 100)
	if got := EstimateTokens(long); got != 100 {
		t.Fatalf("400 chars => %d tokens", got)
	}
	// Muchas palabras cortas: al menos un token por palabra.
	if got := EstimateTokens("a b c d e f"); got < 6 {
		t.Fatalf("6 palabras => %d tokens", got)
	}
	// Unicode cuenta por runas, no por bytes.
	if got := EstimateTokens(strings.Repeat("ñ", 40)); got != 10 {
		t.Fatalf("40 runas ñ => %d", got)
	}
}
