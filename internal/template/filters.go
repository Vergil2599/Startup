package template

import (
	"fmt"
	"strings"
)

// Filter transforma el valor de una variable: {{nombre|filtro}}.
type Filter func(string) string

var filters = map[string]Filter{
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"title": func(s string) string {
		words := strings.Fields(s)
		for i, w := range words {
			r := []rune(w)
			words[i] = strings.ToUpper(string(r[0])) + string(r[1:])
		}
		return strings.Join(words, " ")
	},
	"trim":  strings.TrimSpace,
	"kebab": func(s string) string { return joinWords(s, "-") },
	"snake": func(s string) string { return joinWords(s, "_") },
}

func joinWords(s, sep string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '\t'
	})
	return strings.Join(fields, sep)
}

// RegisterFilter permite a los plugins añadir filtros. Devuelve error si el
// nombre ya existe (los filtros builtin no son reemplazables).
func RegisterFilter(name string, f Filter) error {
	if _, exists := filters[name]; exists {
		return fmt.Errorf("el filtro %q ya está registrado", name)
	}
	filters[name] = f
	return nil
}

func applyFilter(val, name string) (string, error) {
	f, ok := filters[name]
	if !ok {
		return "", fmt.Errorf("filtro desconocido: %q", name)
	}
	return f(val), nil
}
