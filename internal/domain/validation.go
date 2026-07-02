package domain

// Severity gradúa un hallazgo del validador.
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// Finding es un hallazgo concreto de una regla de validación.
type Finding struct {
	RuleID     string
	Severity   Severity
	BlockType  BlockType // vacío si aplica al prompt entero
	Message    string
	Suggestion string
}

// ValidationReport agrega los hallazgos y la puntuación de calidad (0-100).
type ValidationReport struct {
	PromptID ID
	Score    int
	Findings []Finding
	// Breakdown desglosa la penalización aplicada por regla, para que la
	// puntuación sea explicable.
	Breakdown map[string]int
}

// CountBySeverity cuenta los hallazgos de una severidad dada.
func (r ValidationReport) CountBySeverity(s Severity) int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == s {
			n++
		}
	}
	return n
}
