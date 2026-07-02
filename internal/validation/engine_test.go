package validation

import (
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func goodPrompt() *domain.Prompt {
	return &domain.Prompt{
		ID: domain.NewID(), Title: "Bueno",
		Blocks: []domain.Block{
			{Type: domain.BlockRole, Enabled: true, Content: "Eres un revisor senior de Go."},
			{Type: domain.BlockObjective, Enabled: true, Content: "Detectar al menos 3 defectos de corrección por revisión."},
			{Type: domain.BlockOutput, Enabled: true, Content: "Lista en Markdown con archivo y línea."},
		},
	}
}

func findingsFor(t *testing.T, p *domain.Prompt, ruleID string) []domain.Finding {
	t.Helper()
	report := NewEngine().Validate(Input{Prompt: p})
	var out []domain.Finding
	for _, f := range report.Findings {
		if f.RuleID == ruleID {
			out = append(out, f)
		}
	}
	return out
}

func TestGoodPromptScoresHigh(t *testing.T) {
	report := NewEngine().Validate(Input{Prompt: goodPrompt()})
	if report.Score < 90 {
		t.Fatalf("prompt sano puntúa %d, findings: %+v", report.Score, report.Findings)
	}
}

func TestMissingSections(t *testing.T) {
	p := &domain.Prompt{ID: "x", Title: "Vacío"}
	fs := findingsFor(t, p, "missing_sections")
	if len(fs) != 3 { // objective(error) + output(warn) + role(info)
		t.Fatalf("esperaba 3 hallazgos, hay %d: %+v", len(fs), fs)
	}
	// Un Objective desactivado cuenta como ausente.
	p2 := goodPrompt()
	p2.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: false, Content: "algo"})
	fs2 := findingsFor(t, p2, "missing_sections")
	if len(fs2) != 1 || fs2[0].BlockType != domain.BlockObjective {
		t.Fatalf("objective desactivado debe detectarse: %+v", fs2)
	}
}

func TestEmptyBlocks(t *testing.T) {
	p := goodPrompt()
	p.SetBlock(domain.Block{Type: domain.BlockRules, Enabled: true, Content: "  "})
	p.SetBlock(domain.Block{Type: domain.BlockNotes, Enabled: false, Content: ""}) // desactivado: no cuenta
	fs := findingsFor(t, p, "empty_blocks")
	if len(fs) != 1 || fs[0].BlockType != domain.BlockRules {
		t.Fatalf("empty_blocks=%+v", fs)
	}
}

func TestUndefinedVariables(t *testing.T) {
	p := goodPrompt()
	p.SetBlock(domain.Block{Type: domain.BlockContext, Enabled: true, Content: "Proyecto {{project_name}} en {{language}}"})
	report := NewEngine().Validate(Input{
		Prompt: p,
		Vars:   domain.VariableSet{Global: map[string]string{"language": "Go"}},
	})
	var found []domain.Finding
	for _, f := range report.Findings {
		if f.RuleID == "undefined_variables" {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		t.Fatalf("esperaba 1 variable sin definir, hay %d: %+v", len(found), found)
	}
}

func TestDuplicateRules(t *testing.T) {
	p := goodPrompt()
	p.SetBlock(domain.Block{Type: domain.BlockRules, Enabled: true,
		Content: "- No inventes APIs.\n- Usa tests.\n- no inventes apis"})
	fs := findingsFor(t, p, "duplicate_rules")
	if len(fs) != 1 {
		t.Fatalf("duplicate_rules=%+v", fs)
	}
}

func TestContradictions(t *testing.T) {
	p := goodPrompt()
	p.SetBlock(domain.Block{Type: domain.BlockRules, Enabled: true,
		Content: "Nunca uses comentarios.\nSiempre usa comentarios."})
	fs := findingsFor(t, p, "contradictions")
	if len(fs) != 1 {
		t.Fatalf("contradictions=%+v", fs)
	}
	if fs[0].Severity != domain.SeverityError {
		t.Fatal("una contradicción es un error")
	}
}

func TestAmbiguity(t *testing.T) {
	p := goodPrompt()
	p.SetBlock(domain.Block{Type: domain.BlockObjective, Enabled: true, Content: "Mejorar el código."})
	if fs := findingsFor(t, p, "ambiguity"); len(fs) != 1 {
		t.Fatalf("objetivo vago no detectado: %+v", fs)
	}
	// Con Success Criteria definido deja de ser ambiguo.
	p.SetBlock(domain.Block{Type: domain.BlockSuccessCriteria, Enabled: true, Content: "Cobertura > 80%"})
	if fs := findingsFor(t, p, "ambiguity"); len(fs) != 0 {
		t.Fatalf("con criterios medibles no debe haber hallazgo: %+v", fs)
	}
}

func TestClarityCapsAtThree(t *testing.T) {
	long := ""
	for i := 0; i < 60; i++ {
		long += "palabra "
	}
	p := goodPrompt()
	p.SetBlock(domain.Block{Type: domain.BlockContext, Enabled: true,
		Content: long + ". " + long + ". " + long + ". " + long + "."})
	fs := findingsFor(t, p, "clarity")
	if len(fs) != 3 {
		t.Fatalf("clarity debe capar en 3, hay %d", len(fs))
	}
}

func TestScoreMonotonicAndBounded(t *testing.T) {
	e := NewEngine()
	good := e.Validate(Input{Prompt: goodPrompt()})
	bad := e.Validate(Input{Prompt: &domain.Prompt{ID: "x", Title: "Malo"}})
	if bad.Score >= good.Score {
		t.Fatalf("más hallazgos deben bajar el score: bueno=%d malo=%d", good.Score, bad.Score)
	}
	// El desglose suma exactamente la penalización aplicada.
	total := 0
	for _, p := range bad.Breakdown {
		total += p
	}
	if bad.Score != max(0, 100-total) {
		t.Fatalf("score %d no cuadra con desglose %d", bad.Score, total)
	}
	// Peor caso posible: nunca negativo.
	horrible := &domain.Prompt{ID: "h", Title: "H"}
	for _, bt := range domain.BuiltinBlockTypes {
		horrible.SetBlock(domain.Block{Type: bt, Enabled: true, Content: "{{v" + string(bt) + "}}"})
	}
	if r := e.Validate(Input{Prompt: horrible}); r.Score < 0 || r.Score > 100 {
		t.Fatalf("score fuera de rango: %d", r.Score)
	}
}

func TestFindingsSortedBySeverity(t *testing.T) {
	p := &domain.Prompt{ID: "x", Title: "Malo"} // genera error, warn e info
	report := NewEngine().Validate(Input{Prompt: p})
	last := 0
	for _, f := range report.Findings {
		r := sevRank(f.Severity)
		if r < last {
			t.Fatal("hallazgos no ordenados por severidad")
		}
		last = r
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
