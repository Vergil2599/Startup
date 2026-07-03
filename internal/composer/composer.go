// Package composer combina varios prompts/plantillas en capas para producir
// un prompt final (Base + Overlays), con dos estrategias de fusión.
package composer

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

// MergeStrategy define cómo se fusionan los bloques del mismo tipo.
type MergeStrategy string

const (
	// StrategyConcat concatena el contenido de bloques del mismo tipo
	// (capa base primero) separados por línea en blanco.
	StrategyConcat MergeStrategy = "concat"
	// StrategyReplace hace que la capa superior reemplace al bloque completo.
	StrategyReplace MergeStrategy = "replace"
)

// Layer es una capa de la composición.
type Layer struct {
	Ref     string `yaml:"ref"` // ID o ruta relativa .md
	Enabled bool   `yaml:"enabled"`
}

// Composition es la definición declarativa (compositions/*.yaml).
type Composition struct {
	Title    string        `yaml:"title"`
	Strategy MergeStrategy `yaml:"strategy"`
	Layers   []Layer       `yaml:"layers"`
}

// ParseComposition lee una composición YAML.
func ParseComposition(data []byte) (*Composition, error) {
	var c Composition
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("composición inválida: %w", err)
	}
	if c.Title == "" || len(c.Layers) == 0 {
		return nil, fmt.Errorf("composición inválida: se requieren title y layers")
	}
	if c.Strategy == "" {
		c.Strategy = StrategyConcat
	}
	if c.Strategy != StrategyConcat && c.Strategy != StrategyReplace {
		return nil, fmt.Errorf("estrategia desconocida: %q", c.Strategy)
	}
	return &c, nil
}

// Resolver carga un prompt por referencia (ID o ruta).
type Resolver func(ref string) (*domain.Prompt, error)

// Compose materializa la composición en un prompt nuevo. El orden de capas
// importa: la primera es la base, las siguientes se apilan encima.
func Compose(c *Composition, resolve Resolver) (*domain.Prompt, error) {
	now := time.Now().UTC().Truncate(time.Second)
	out := &domain.Prompt{
		ID: domain.NewID(), Title: c.Title, Schema: domain.SchemaVersion,
		Variables: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	loaded := 0
	for _, layer := range c.Layers {
		if !layer.Enabled {
			continue
		}
		p, err := resolve(layer.Ref)
		if err != nil {
			return nil, fmt.Errorf("capa %q: %w", layer.Ref, err)
		}
		loaded++
		for _, b := range p.Blocks {
			if !b.Enabled {
				continue
			}
			existing, ok := out.Block(b.Type)
			switch {
			case !ok:
				out.Blocks = append(out.Blocks, domain.Block{Type: b.Type, Enabled: true, Content: b.Content})
			case c.Strategy == StrategyReplace:
				out.SetBlock(domain.Block{Type: b.Type, Enabled: true, Content: b.Content})
			default: // concat
				merged := strings.TrimRight(existing.Content, "\n")
				if merged != "" && strings.TrimSpace(b.Content) != "" {
					merged += "\n\n"
				}
				merged += strings.TrimRight(b.Content, "\n")
				out.SetBlock(domain.Block{Type: b.Type, Enabled: true, Content: merged})
			}
		}
		for k, v := range p.Variables {
			out.Variables[k] = v // la capa superior gana
		}
		for _, t := range p.Tags {
			out.Tags = append(out.Tags, t)
		}
	}
	if loaded == 0 {
		return nil, fmt.Errorf("la composición no tiene capas activas")
	}
	out.Tags = domain.NormalizeTags(out.Tags)
	return out, nil
}
