package domain

import "regexp"

// VariableScope indica dónde vive una variable. La precedencia de resolución es
// prompt > proyecto > global (el ámbito más cercano gana).
type VariableScope string

const (
	ScopeGlobal  VariableScope = "global"
	ScopeProject VariableScope = "project"
	ScopePrompt  VariableScope = "prompt"
)

// Variable es un valor reutilizable referenciable como {{nombre}}.
type Variable struct {
	Name        string
	Value       string
	Description string
	Scope       VariableScope
	Secret      bool
}

var varNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ValidVariableName valida el formato snake_case exigido a los nombres.
func ValidVariableName(name string) bool { return varNameRe.MatchString(name) }

// VariableSet resuelve variables por capas de ámbito.
type VariableSet struct {
	Global  map[string]string
	Project map[string]string
	Prompt  map[string]string
}

// Resolve busca una variable respetando la precedencia prompt > proyecto > global.
func (v VariableSet) Resolve(name string) (string, bool) {
	for _, m := range []map[string]string{v.Prompt, v.Project, v.Global} {
		if m != nil {
			if val, ok := m[name]; ok {
				return val, true
			}
		}
	}
	return "", false
}
