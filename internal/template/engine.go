// Package template implementa el motor de plantillas de PES: interpolación de
// variables {{nombre}} con filtros ({{nombre|upper}}), extracción de variables
// y herencia de plantillas. El contenido de una variable nunca se re-interpreta
// como sintaxis (sin inyección).
package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

var placeholderRe = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]*)\s*(?:\|\s*([a-z][a-z0-9_]*)\s*)?\}\}`)

// RenderOpts controla el render.
type RenderOpts struct {
	// Strict hace fallar el render si queda alguna variable sin resolver.
	Strict bool
	// BlockSeparator separa los bloques en la salida (por defecto "\n\n").
	BlockSeparator string
	// IncludeHeadings antepone "## Tipo" a cada bloque en la salida.
	IncludeHeadings bool
}

// RenderString interpola las variables de un texto usando el conjunto dado.
// Devuelve el texto resultante y la lista de variables sin resolver.
func RenderString(s string, vars domain.VariableSet) (string, []string) {
	var missing []string
	out := placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		g := placeholderRe.FindStringSubmatch(m)
		name, filter := g[1], g[2]
		val, ok := vars.Resolve(name)
		if !ok {
			missing = append(missing, name)
			return m // se conserva el placeholder para que sea visible
		}
		if filter != "" {
			fv, err := applyFilter(val, filter)
			if err != nil {
				missing = append(missing, name+"|"+filter)
				return m
			}
			val = fv
		}
		return val
	})
	return out, dedupe(missing)
}

// RenderPrompt renderiza los bloques activos de un prompt en un texto final.
func RenderPrompt(p *domain.Prompt, vars domain.VariableSet, opts RenderOpts) (string, error) {
	if opts.BlockSeparator == "" {
		opts.BlockSeparator = "\n\n"
	}
	// Las variables locales del prompt tienen la máxima precedencia.
	if len(p.Variables) > 0 {
		merged := domain.VariableSet{Global: vars.Global, Project: vars.Project}
		mp := map[string]string{}
		for k, v := range vars.Prompt {
			mp[k] = v
		}
		for k, v := range p.Variables {
			mp[k] = v
		}
		merged.Prompt = mp
		vars = merged
	}
	var parts []string
	var allMissing []string
	for _, b := range p.EnabledBlocks() {
		if b.IsEmpty() {
			continue
		}
		body, missing := RenderString(b.Content, vars)
		allMissing = append(allMissing, missing...)
		if opts.IncludeHeadings {
			body = "## " + headingFor(b.Type) + "\n" + body
		}
		parts = append(parts, strings.TrimRight(body, "\n"))
	}
	out := strings.Join(parts, opts.BlockSeparator)
	if opts.Strict && len(allMissing) > 0 {
		return out, fmt.Errorf("%w: %s", domain.ErrUnresolvedVars, strings.Join(dedupe(allMissing), ", "))
	}
	return out, nil
}

// ExtractVariables devuelve los nombres de variables referenciados en un prompt
// (bloques activos e inactivos, para que el panel de variables sea completo).
func ExtractVariables(p *domain.Prompt) []string {
	var names []string
	for _, b := range p.Blocks {
		for _, g := range placeholderRe.FindAllStringSubmatch(b.Content, -1) {
			names = append(names, g[1])
		}
	}
	return dedupe(names)
}

// UnresolvedVariables devuelve las variables usadas en bloques activos que no
// resuelven con el conjunto dado (insumo de la regla de validación).
func UnresolvedVariables(p *domain.Prompt, vars domain.VariableSet) []string {
	if len(p.Variables) > 0 {
		mp := map[string]string{}
		for k, v := range vars.Prompt {
			mp[k] = v
		}
		for k, v := range p.Variables {
			mp[k] = v
		}
		vars = domain.VariableSet{Global: vars.Global, Project: vars.Project, Prompt: mp}
	}
	var missing []string
	for _, b := range p.EnabledBlocks() {
		for _, g := range placeholderRe.FindAllStringSubmatch(b.Content, -1) {
			if _, ok := vars.Resolve(g[1]); !ok {
				missing = append(missing, g[1])
			}
		}
	}
	return dedupe(missing)
}

func headingFor(t domain.BlockType) string {
	words := strings.Split(string(t), "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
