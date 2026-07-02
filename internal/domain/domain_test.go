package domain

import (
	"strings"
	"testing"
)

func TestNewIDUniqueAndSortable(t *testing.T) {
	a, b := NewID(), NewID()
	if a == b {
		t.Fatal("dos IDs consecutivos no deben coincidir")
	}
	if len(a) != 26 {
		t.Fatalf("ULID debe tener 26 caracteres, tiene %d", len(a))
	}
}

func TestPromptValidate(t *testing.T) {
	cases := []struct {
		name    string
		prompt  Prompt
		wantErr error
	}{
		{"válido", Prompt{Title: "x", Blocks: []Block{{Type: BlockRole, Enabled: true}}}, nil},
		{"sin título", Prompt{Title: "  "}, ErrEmptyTitle},
		{"bloque duplicado", Prompt{Title: "x", Blocks: []Block{{Type: BlockRole}, {Type: BlockRole}}}, ErrDuplicateBlock},
		{"tipo vacío", Prompt{Title: "x", Blocks: []Block{{Type: " "}}}, ErrInvalidBlockType},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.prompt.Validate(); err != c.wantErr {
				t.Fatalf("Validate() = %v, quería %v", err, c.wantErr)
			}
		})
	}
}

func TestEnabledBlocksFiltersAndKeepsOrder(t *testing.T) {
	p := Prompt{Title: "x", Blocks: []Block{
		{Type: BlockRole, Enabled: true, Content: "a"},
		{Type: BlockRules, Enabled: false, Content: "b"},
		{Type: BlockOutput, Enabled: true, Content: "c"},
	}}
	got := p.EnabledBlocks()
	if len(got) != 2 || got[0].Type != BlockRole || got[1].Type != BlockOutput {
		t.Fatalf("EnabledBlocks() = %+v", got)
	}
}

func TestSetBlockReplacesOrAppends(t *testing.T) {
	p := Prompt{Title: "x"}
	p.SetBlock(Block{Type: BlockRole, Content: "v1"})
	p.SetBlock(Block{Type: BlockRole, Content: "v2"})
	p.SetBlock(Block{Type: BlockNotes, Content: "n"})
	if len(p.Blocks) != 2 {
		t.Fatalf("esperaba 2 bloques, hay %d", len(p.Blocks))
	}
	if b, _ := p.Block(BlockRole); b.Content != "v2" {
		t.Fatalf("SetBlock no reemplazó: %q", b.Content)
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Go ", "go", "", "Code-Review"})
	want := []string{"go", "code-review"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("NormalizeTags = %v, quería %v", got, want)
	}
}

func TestVariableSetPrecedence(t *testing.T) {
	vs := VariableSet{
		Global:  map[string]string{"lang": "python", "db": "sqlite"},
		Project: map[string]string{"lang": "go"},
		Prompt:  map[string]string{"lang": "rust"},
	}
	if v, _ := vs.Resolve("lang"); v != "rust" {
		t.Fatalf("precedencia prompt debe ganar, obtuve %q", v)
	}
	if v, _ := vs.Resolve("db"); v != "sqlite" {
		t.Fatalf("fallback a global roto, obtuve %q", v)
	}
	if _, ok := vs.Resolve("nope"); ok {
		t.Fatal("variable inexistente no debe resolverse")
	}
}

func TestValidVariableName(t *testing.T) {
	valid := []string{"project_name", "llm", "a1_b2"}
	invalid := []string{"ProjectName", "1abc", "a-b", "", "_x", "á"}
	for _, n := range valid {
		if !ValidVariableName(n) {
			t.Errorf("%q debería ser válido", n)
		}
	}
	for _, n := range invalid {
		if ValidVariableName(n) {
			t.Errorf("%q debería ser inválido", n)
		}
	}
}

func TestIsBuiltinBlockType(t *testing.T) {
	if !IsBuiltinBlockType(BlockObjective) {
		t.Fatal("objective es builtin")
	}
	if IsBuiltinBlockType("custom_thing") {
		t.Fatal("custom_thing no es builtin")
	}
}

func TestReportCountBySeverity(t *testing.T) {
	r := ValidationReport{Findings: []Finding{
		{Severity: SeverityError}, {Severity: SeverityWarn}, {Severity: SeverityError},
	}}
	if r.CountBySeverity(SeverityError) != 2 || r.CountBySeverity(SeverityInfo) != 0 {
		t.Fatal("CountBySeverity incorrecto")
	}
}
