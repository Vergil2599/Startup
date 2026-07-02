// Package validation implementa el validador determinista de PES: un pipeline
// de reglas registrables que produce hallazgos y una puntuación 0-100
// explicable. No usa IA y funciona 100 % offline.
package validation

import (
	"sort"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

// Input agrupa todo lo que una regla puede necesitar.
type Input struct {
	Prompt *domain.Prompt
	Vars   domain.VariableSet
}

// Rule es una regla de validación. Los plugins registran las suyas.
type Rule interface {
	ID() string
	Check(in Input) []domain.Finding
}

// Penalizaciones por severidad para la puntuación.
var penalty = map[domain.Severity]int{
	domain.SeverityError: 15,
	domain.SeverityWarn:  7,
	domain.SeverityInfo:  2,
}

// Engine ejecuta el pipeline de reglas.
type Engine struct {
	rules []Rule
}

// NewEngine crea un engine con las reglas incorporadas.
func NewEngine() *Engine {
	return &Engine{rules: []Rule{
		MissingSectionsRule{},
		EmptyBlocksRule{},
		UndefinedVariablesRule{},
		DuplicateRulesRule{},
		ContradictionsRule{},
		AmbiguityRule{},
		ClarityRule{},
	}}
}

// Register añade una regla externa (plugin). Silenciosamente ignora IDs duplicados.
func (e *Engine) Register(r Rule) {
	for _, existing := range e.rules {
		if existing.ID() == r.ID() {
			return
		}
	}
	e.rules = append(e.rules, r)
}

// Validate ejecuta todas las reglas y calcula la puntuación.
// La puntuación parte de 100 y resta por hallazgo según severidad; es
// monotónica (más hallazgos nunca suben el score) y su desglose por regla
// suma exactamente la penalización aplicada.
func (e *Engine) Validate(in Input) domain.ValidationReport {
	report := domain.ValidationReport{
		PromptID:  in.Prompt.ID,
		Breakdown: map[string]int{},
	}
	for _, r := range e.rules {
		findings := r.Check(in)
		for _, f := range findings {
			f.RuleID = r.ID()
			report.Findings = append(report.Findings, f)
			report.Breakdown[r.ID()] += penalty[f.Severity]
		}
	}
	total := 0
	for _, p := range report.Breakdown {
		total += p
	}
	score := 100 - total
	if score < 0 {
		score = 0
	}
	report.Score = score
	sort.SliceStable(report.Findings, func(i, j int) bool {
		return sevRank(report.Findings[i].Severity) < sevRank(report.Findings[j].Severity)
	})
	return report
}

func sevRank(s domain.Severity) int {
	switch s {
	case domain.SeverityError:
		return 0
	case domain.SeverityWarn:
		return 1
	default:
		return 2
	}
}
