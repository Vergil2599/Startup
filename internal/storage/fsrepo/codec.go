// Package fsrepo implementa la fuente de verdad de PES: prompts y plantillas
// como archivos Markdown con front-matter YAML, en una carpeta del usuario.
package fsrepo

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

// fileMeta es el front-matter YAML del archivo de prompt.
type fileMeta struct {
	Pes         int               `yaml:"pes"`
	ID          string            `yaml:"id"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	Category    string            `yaml:"category,omitempty"`
	Favorite    bool              `yaml:"favorite,omitempty"`
	Template    string            `yaml:"template,omitempty"`
	Extends     string            `yaml:"extends,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	Blocks      []blockMeta       `yaml:"blocks"`
	Created     time.Time         `yaml:"created"`
	Updated     time.Time         `yaml:"updated"`
}

type blockMeta struct {
	Type    string `yaml:"type"`
	Enabled bool   `yaml:"enabled"`
}

var blockHeadingRe = regexp.MustCompile(`(?m)^## @([a-z][a-z0-9_]*)\s*$`)

// Serialize convierte un prompt a su representación de archivo (.md).
func Serialize(p *domain.Prompt) ([]byte, error) {
	meta := fileMeta{
		Pes: p.Schema, ID: string(p.ID), Title: p.Title, Description: p.Description,
		Tags: p.Tags, Category: p.Category, Favorite: p.Favorite,
		Template: string(p.TemplateID), Extends: string(p.Extends),
		Variables: p.Variables, Created: p.CreatedAt.UTC(), Updated: p.UpdatedAt.UTC(),
	}
	if meta.Pes == 0 {
		meta.Pes = domain.SchemaVersion
	}
	for _, b := range p.Blocks {
		meta.Blocks = append(meta.Blocks, blockMeta{Type: string(b.Type), Enabled: b.Enabled})
	}
	fm, err := yaml.Marshal(meta)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fm)
	sb.WriteString("---\n")
	for _, b := range p.Blocks {
		sb.WriteString("\n## @")
		sb.WriteString(string(b.Type))
		sb.WriteString("\n")
		content := strings.TrimRight(b.Content, "\n")
		if content != "" {
			sb.WriteString(content)
			sb.WriteString("\n")
		}
	}
	return []byte(sb.String()), nil
}

// Parse reconstruye un prompt desde el contenido de un archivo .md.
func Parse(data []byte) (*domain.Prompt, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, fmt.Errorf("%w: falta el front-matter", domain.ErrInvalidFileFormat)
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, fmt.Errorf("%w: front-matter sin cerrar", domain.ErrInvalidFileFormat)
	}
	var meta fileMeta
	if err := yaml.Unmarshal([]byte(rest[:end+1]), &meta); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidFileFormat, err)
	}
	if meta.ID == "" || strings.TrimSpace(meta.Title) == "" {
		return nil, fmt.Errorf("%w: id o title ausentes", domain.ErrInvalidFileFormat)
	}
	body := rest[end+5:]
	contents := parseBlocks(body)

	p := &domain.Prompt{
		ID: domain.ID(meta.ID), Title: meta.Title, Description: meta.Description,
		Tags: domain.NormalizeTags(meta.Tags), Category: meta.Category,
		Favorite: meta.Favorite, TemplateID: domain.ID(meta.Template),
		Extends: domain.ID(meta.Extends), Variables: meta.Variables,
		CreatedAt: meta.Created, UpdatedAt: meta.Updated, Schema: meta.Pes,
	}
	seen := map[string]bool{}
	for _, bm := range meta.Blocks {
		seen[bm.Type] = true
		p.Blocks = append(p.Blocks, domain.Block{
			Type: domain.BlockType(bm.Type), Enabled: bm.Enabled, Content: contents[bm.Type],
		})
	}
	// Bloques presentes en el cuerpo pero no declarados: se conservan activos
	// (tolerancia con ediciones manuales externas).
	for _, m := range blockHeadingRe.FindAllStringSubmatch(body, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			p.Blocks = append(p.Blocks, domain.Block{
				Type: domain.BlockType(m[1]), Enabled: true, Content: contents[m[1]],
			})
		}
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidFileFormat, err)
	}
	return p, nil
}

func parseBlocks(body string) map[string]string {
	out := map[string]string{}
	locs := blockHeadingRe.FindAllStringSubmatchIndex(body, -1)
	for i, loc := range locs {
		name := body[loc[2]:loc[3]]
		start := loc[1]
		endPos := len(body)
		if i+1 < len(locs) {
			endPos = locs[i+1][0]
		}
		out[name] = strings.Trim(body[start:endPos], "\n")
	}
	return out
}
