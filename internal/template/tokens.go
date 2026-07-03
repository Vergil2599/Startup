package template

import "unicode/utf8"

// EstimateTokens aproxima los tokens LLM de un texto sin depender de ningún
// tokenizador concreto: ~4 caracteres por token, con un mínimo de un token
// por palabra corta. Es una estimación orientativa (±15 % típico frente a
// tokenizadores BPE reales), suficiente para presupuestar prompts.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	runes := utf8.RuneCountInString(s)
	words := 0
	inWord := false
	for _, r := range s {
		space := r == ' ' || r == '\n' || r == '\t' || r == '\r'
		if !space && !inWord {
			words++
		}
		inWord = !space
	}
	byChars := (runes + 3) / 4
	if words > byChars {
		return words
	}
	return byChars
}
