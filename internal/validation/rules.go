package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/template"
)

// MissingSectionsRule: secciones esenciales ausentes o desactivadas.
type MissingSectionsRule struct{}

func (MissingSectionsRule) ID() string { return "missing_sections" }

func (MissingSectionsRule) Check(in Input) []domain.Finding {
	var out []domain.Finding
	checks := []struct {
		t   domain.BlockType
		sev domain.Severity
		msg string
	}{
		{domain.BlockObjective, domain.SeverityError, "el prompt no tiene un Objective activo: el modelo no sabrá qué debe lograr"},
		{domain.BlockOutput, domain.SeverityWarn, "no hay bloque Output activo: el formato de la respuesta queda indefinido"},
		{domain.BlockRole, domain.SeverityInfo, "definir un Role suele mejorar la consistencia de la respuesta"},
	}
	for _, c := range checks {
		b, ok := in.Prompt.Block(c.t)
		if !ok || !b.Enabled || b.IsEmpty() {
			out = append(out, domain.Finding{
				Severity: c.sev, BlockType: c.t, Message: c.msg,
				Suggestion: fmt.Sprintf("añade y activa un bloque %q con contenido", c.t),
			})
		}
	}
	return out
}

// EmptyBlocksRule: bloques activos sin contenido.
type EmptyBlocksRule struct{}

func (EmptyBlocksRule) ID() string { return "empty_blocks" }

func (EmptyBlocksRule) Check(in Input) []domain.Finding {
	var out []domain.Finding
	for _, b := range in.Prompt.EnabledBlocks() {
		// Los bloques cuya ausencia ya penaliza MissingSectionsRule no se duplican aquí.
		if b.Type == domain.BlockObjective || b.Type == domain.BlockOutput || b.Type == domain.BlockRole {
			continue
		}
		if b.IsEmpty() {
			out = append(out, domain.Finding{
				Severity: domain.SeverityWarn, BlockType: b.Type,
				Message:    fmt.Sprintf("el bloque %q está activo pero vacío", b.Type),
				Suggestion: "rellénalo o desactívalo",
			})
		}
	}
	return out
}

// UndefinedVariablesRule: variables usadas en bloques activos sin valor en ningún ámbito.
type UndefinedVariablesRule struct{}

func (UndefinedVariablesRule) ID() string { return "undefined_variables" }

func (UndefinedVariablesRule) Check(in Input) []domain.Finding {
	var out []domain.Finding
	for _, name := range template.UnresolvedVariables(in.Prompt, in.Vars) {
		out = append(out, domain.Finding{
			Severity:   domain.SeverityError,
			Message:    fmt.Sprintf("la variable {{%s}} no está definida en ningún ámbito", name),
			Suggestion: fmt.Sprintf("define %q como variable global, de proyecto o del prompt", name),
		})
	}
	return out
}

// DuplicateRulesRule: líneas repetidas dentro de Rules/Constraints.
type DuplicateRulesRule struct{}

func (DuplicateRulesRule) ID() string { return "duplicate_rules" }

func (DuplicateRulesRule) Check(in Input) []domain.Finding {
	var out []domain.Finding
	for _, t := range []domain.BlockType{domain.BlockRules, domain.BlockConstraints} {
		b, ok := in.Prompt.Block(t)
		if !ok || !b.Enabled {
			continue
		}
		seen := map[string]bool{}
		for _, line := range strings.Split(b.Content, "\n") {
			norm := normalizeLine(line)
			if norm == "" {
				continue
			}
			if seen[norm] {
				out = append(out, domain.Finding{
					Severity: domain.SeverityWarn, BlockType: t,
					Message:    fmt.Sprintf("regla repetida en %q: %q", t, strings.TrimSpace(line)),
					Suggestion: "elimina la repetición; repetir reglas no las refuerza",
				})
			}
			seen[norm] = true
		}
	}
	return out
}

// ContradictionsRule: heurística léxica "nunca X" vs "siempre X" (es/en).
type ContradictionsRule struct{}

func (ContradictionsRule) ID() string { return "contradictions" }

var neverRe = regexp.MustCompile(`(?i)\b(?:nunca|jamás|jamas|never|do not|don't|no debes?)\s+(.{3,60}?)(?:[.,;\n]|$)`)
var alwaysRe = regexp.MustCompile(`(?i)\b(?:siempre|always|debes?)\s+(.{3,60}?)(?:[.,;\n]|$)`)

func (ContradictionsRule) Check(in Input) []domain.Finding {
	text := joinEnabled(in.Prompt)
	never := map[string]bool{}
	for _, m := range neverRe.FindAllStringSubmatch(text, -1) {
		for _, k := range phraseKeys(m[1]) {
			never[k] = true
		}
	}
	var out []domain.Finding
	seen := map[string]bool{}
	for _, m := range alwaysRe.FindAllStringSubmatch(text, -1) {
		for _, k := range phraseKeys(m[1]) {
			if never[k] && !seen[k] {
				seen[k] = true
				out = append(out, domain.Finding{
					Severity:   domain.SeverityError,
					Message:    fmt.Sprintf("posible contradicción: se prohíbe y se exige %q a la vez", k),
					Suggestion: "decide una sola política para esa acción",
				})
			}
		}
	}
	return out
}

// phraseKeys genera claves de comparación para una frase: la frase completa
// normalizada y, si tiene más de una palabra, la frase sin el verbo inicial
// (para que "nunca uses X" y "siempre usa X" coincidan pese a la conjugación).
func phraseKeys(phrase string) []string {
	norm := normalizeLine(phrase)
	if norm == "" {
		return nil
	}
	keys := []string{norm}
	if words := strings.Fields(norm); len(words) > 1 {
		keys = append(keys, strings.Join(words[1:], " "))
	}
	return keys
}

// AmbiguityRule: el Objective usa verbos vagos sin criterio medible.
type AmbiguityRule struct{}

func (AmbiguityRule) ID() string { return "ambiguity" }

var vagueVerbs = []string{
	"mejorar", "optimizar", "ayudar", "apoyar", "potenciar", "enriquecer",
	"improve", "optimize", "help", "enhance", "assist", "support",
}
var measurableRe = regexp.MustCompile(`(?i)\d|%|\bmétrica|\bmetric|\bcriteri|\bkpi\b|\bmedib`)

func (AmbiguityRule) Check(in Input) []domain.Finding {
	b, ok := in.Prompt.Block(domain.BlockObjective)
	if !ok || !b.Enabled || b.IsEmpty() {
		return nil
	}
	lower := strings.ToLower(b.Content)
	hasCriteria := measurableRe.MatchString(b.Content)
	if _, defined := in.Prompt.Block(domain.BlockSuccessCriteria); defined {
		if sc, _ := in.Prompt.Block(domain.BlockSuccessCriteria); sc.Enabled && !sc.IsEmpty() {
			hasCriteria = true
		}
	}
	var out []domain.Finding
	for _, v := range vagueVerbs {
		if strings.Contains(lower, v) && !hasCriteria {
			out = append(out, domain.Finding{
				Severity: domain.SeverityWarn, BlockType: domain.BlockObjective,
				Message:    fmt.Sprintf("el objetivo usa el verbo vago %q sin criterio medible", v),
				Suggestion: "añade una métrica o un bloque Success Criteria",
			})
			break // un solo hallazgo por prompt: evitar castigo acumulado
		}
	}
	return out
}

// ClarityRule: frases excesivamente largas dificultan el seguimiento.
type ClarityRule struct{}

func (ClarityRule) ID() string { return "clarity" }

const maxSentenceWords = 45

func (ClarityRule) Check(in Input) []domain.Finding {
	var out []domain.Finding
	for _, b := range in.Prompt.EnabledBlocks() {
		for _, sentence := range strings.FieldsFunc(b.Content, func(r rune) bool {
			return r == '.' || r == '\n' || r == '!' || r == '?'
		}) {
			if len(strings.Fields(sentence)) > maxSentenceWords {
				out = append(out, domain.Finding{
					Severity: domain.SeverityInfo, BlockType: b.Type,
					Message:    fmt.Sprintf("frase de más de %d palabras en %q", maxSentenceWords, b.Type),
					Suggestion: "divídela en frases o viñetas más cortas",
				})
				if len(out) >= 3 {
					return out // cap: no castigar en cascada
				}
			}
		}
	}
	return out
}

func joinEnabled(p *domain.Prompt) string {
	var sb strings.Builder
	for _, b := range p.EnabledBlocks() {
		sb.WriteString(b.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

var spaceRe = regexp.MustCompile(`\s+`)

func normalizeLine(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimLeft(s, "-*•· \t0123456789.)")
	s = strings.TrimRight(s, ".,;: \t")
	return spaceRe.ReplaceAllString(s, " ")
}
