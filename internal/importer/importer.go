// Package importer detecta el formato de un archivo y lo convierte en prompt.
package importer

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// structured refleja el formato de export JSON/YAML.
type structured struct {
	ID          string            `json:"id" yaml:"id"`
	Title       string            `json:"title" yaml:"title"`
	Description string            `json:"description" yaml:"description"`
	Tags        []string          `json:"tags" yaml:"tags"`
	Category    string            `json:"category" yaml:"category"`
	Favorite    bool              `json:"favorite" yaml:"favorite"`
	Variables   map[string]string `json:"variables" yaml:"variables"`
	Blocks      []struct {
		Type    string `json:"type" yaml:"type"`
		Enabled bool   `json:"enabled" yaml:"enabled"`
		Content string `json:"content" yaml:"content"`
	} `json:"blocks" yaml:"blocks"`
}

// Import detecta el formato (pes-md, json, yaml, texto plano) y devuelve el prompt.
// Para texto plano crea un prompt con el contenido en un bloque Context.
func Import(data []byte, fallbackTitle string) (*domain.Prompt, error) {
	trimmed := strings.TrimSpace(string(data))
	switch {
	case strings.HasPrefix(trimmed, "---"):
		p, err := fsrepo.Parse(data)
		if err == nil {
			return withID(p), nil
		}
		// front-matter presente pero no es formato PES: cae a texto plano
	case strings.HasPrefix(trimmed, "{"):
		var s structured
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("JSON inválido: %w", err)
		}
		return fromStructured(s)
	default:
		var s structured
		if err := yaml.Unmarshal(data, &s); err == nil && s.Title != "" && len(s.Blocks) > 0 {
			return fromStructured(s)
		}
	}
	if trimmed == "" {
		return nil, fmt.Errorf("el archivo está vacío")
	}
	if fallbackTitle == "" {
		fallbackTitle = "Prompt importado"
	}
	p := &domain.Prompt{
		ID: domain.NewID(), Title: fallbackTitle, Schema: domain.SchemaVersion,
		Blocks: []domain.Block{{Type: domain.BlockContext, Enabled: true, Content: trimmed}},
	}
	return p, nil
}

func fromStructured(s structured) (*domain.Prompt, error) {
	p := &domain.Prompt{
		ID: domain.ID(s.ID), Title: s.Title, Description: s.Description,
		Tags: domain.NormalizeTags(s.Tags), Category: s.Category, Favorite: s.Favorite,
		Variables: s.Variables, Schema: domain.SchemaVersion,
	}
	for _, b := range s.Blocks {
		p.Blocks = append(p.Blocks, domain.Block{Type: domain.BlockType(b.Type), Enabled: b.Enabled, Content: b.Content})
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return withID(p), nil
}

// withID garantiza un ID nuevo si el archivo importado no traía uno.
func withID(p *domain.Prompt) *domain.Prompt {
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	return p
}
