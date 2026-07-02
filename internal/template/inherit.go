package template

import (
	"github.com/Vergil2599/startup/pes/internal/domain"
)

// TemplateLookup resuelve una plantilla por ID (lo implementa el repositorio).
type TemplateLookup func(id domain.ID) (*domain.Prompt, error)

// ResolveInheritance materializa la cadena de herencia de una plantilla:
// los bloques del hijo sobrescriben a los del padre (mismo tipo); los bloques
// del padre no redefinidos se heredan manteniendo el orden del padre primero.
// Las variables locales se fusionan con precedencia del hijo. Detecta ciclos.
func ResolveInheritance(tpl *domain.Prompt, lookup TemplateLookup) (*domain.Prompt, error) {
	visited := map[domain.ID]bool{tpl.ID: true}
	chain := []*domain.Prompt{tpl}
	cur := tpl
	for cur.Extends != "" {
		if visited[cur.Extends] {
			return nil, domain.ErrTemplateCycle
		}
		parent, err := lookup(cur.Extends)
		if err != nil {
			return nil, err
		}
		visited[parent.ID] = true
		chain = append(chain, parent)
		cur = parent
	}
	// Fusionar de la raíz hacia el hijo.
	out := &domain.Prompt{
		ID: tpl.ID, Title: tpl.Title, Description: tpl.Description,
		Tags: tpl.Tags, Category: tpl.Category, Schema: tpl.Schema,
		Variables: map[string]string{},
	}
	for i := len(chain) - 1; i >= 0; i-- {
		layer := chain[i]
		for _, b := range layer.Blocks {
			out.SetBlock(b)
		}
		for k, v := range layer.Variables {
			out.Variables[k] = v
		}
		if out.Category == "" {
			out.Category = layer.Category
		}
	}
	return out, nil
}
