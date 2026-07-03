// Package export implementa los codecs de exportación de prompts.
// El registro es extensible: los plugins podrán aportar formatos nuevos.
package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/template"
)

// Input agrupa lo necesario para exportar.
type Input struct {
	Prompt *domain.Prompt
	Vars   domain.VariableSet
}

// Codec convierte un prompt a un formato de salida.
type Codec interface {
	ID() string        // "md", "json"…
	Extension() string // ".md"
	Export(in Input) ([]byte, error)
}

var registry = map[string]Codec{}

// Register añade un codec; error si el ID ya existe.
func Register(c Codec) error {
	if _, dup := registry[c.ID()]; dup {
		return fmt.Errorf("codec %q ya registrado", c.ID())
	}
	registry[c.ID()] = c
	return nil
}

// Get devuelve el codec de un formato.
func Get(id string) (Codec, error) {
	c, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("formato de exportación desconocido: %q (disponibles: %v)", id, Formats())
	}
	return c, nil
}

// Formats lista los formatos registrados, ordenados.
func Formats() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func init() {
	for _, c := range []Codec{mdCodec{}, txtCodec{}, jsonCodec{}, yamlCodec{}, htmlCodec{}, pdfCodec{}} {
		if err := Register(c); err != nil {
			panic(err)
		}
	}
}

// ── Codecs de texto renderizado ──────────────────────────────────────────────

// mdCodec exporta el prompt renderizado como Markdown con encabezados por bloque.
type mdCodec struct{}

func (mdCodec) ID() string        { return "md" }
func (mdCodec) Extension() string { return ".md" }
func (mdCodec) Export(in Input) ([]byte, error) {
	s, err := template.RenderPrompt(in.Prompt, in.Vars, template.RenderOpts{IncludeHeadings: true})
	if err != nil {
		return nil, err
	}
	return []byte("# " + in.Prompt.Title + "\n\n" + s + "\n"), nil
}

// txtCodec exporta el prompt renderizado como texto plano listo para pegar.
type txtCodec struct{}

func (txtCodec) ID() string        { return "txt" }
func (txtCodec) Extension() string { return ".txt" }
func (txtCodec) Export(in Input) ([]byte, error) {
	s, err := template.RenderPrompt(in.Prompt, in.Vars, template.RenderOpts{})
	if err != nil {
		return nil, err
	}
	return []byte(s + "\n"), nil
}

// ── Codecs estructurados (conservan bloques y metadatos) ─────────────────────

type structured struct {
	Pes         int               `json:"pes" yaml:"pes"`
	ID          string            `json:"id" yaml:"id"`
	Title       string            `json:"title" yaml:"title"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Category    string            `json:"category,omitempty" yaml:"category,omitempty"`
	Favorite    bool              `json:"favorite,omitempty" yaml:"favorite,omitempty"`
	Variables   map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	Blocks      []structuredBlock `json:"blocks" yaml:"blocks"`
	Created     time.Time         `json:"created" yaml:"created"`
	Updated     time.Time         `json:"updated" yaml:"updated"`
}

type structuredBlock struct {
	Type    string `json:"type" yaml:"type"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Content string `json:"content" yaml:"content"`
}

func toStructured(p *domain.Prompt) structured {
	s := structured{
		Pes: p.Schema, ID: string(p.ID), Title: p.Title, Description: p.Description,
		Tags: p.Tags, Category: p.Category, Favorite: p.Favorite,
		Variables: p.Variables, Created: p.CreatedAt, Updated: p.UpdatedAt,
	}
	if s.Pes == 0 {
		s.Pes = domain.SchemaVersion
	}
	for _, b := range p.Blocks {
		s.Blocks = append(s.Blocks, structuredBlock{Type: string(b.Type), Enabled: b.Enabled, Content: b.Content})
	}
	return s
}

// FromStructured reconstruye un prompt desde la forma estructurada (import).
func FromStructured(s structured) *domain.Prompt {
	p := &domain.Prompt{
		ID: domain.ID(s.ID), Title: s.Title, Description: s.Description,
		Tags: domain.NormalizeTags(s.Tags), Category: s.Category, Favorite: s.Favorite,
		Variables: s.Variables, CreatedAt: s.Created, UpdatedAt: s.Updated, Schema: s.Pes,
	}
	for _, b := range s.Blocks {
		p.Blocks = append(p.Blocks, domain.Block{Type: domain.BlockType(b.Type), Enabled: b.Enabled, Content: b.Content})
	}
	return p
}

type jsonCodec struct{}

func (jsonCodec) ID() string        { return "json" }
func (jsonCodec) Extension() string { return ".json" }
func (jsonCodec) Export(in Input) ([]byte, error) {
	return json.MarshalIndent(toStructured(in.Prompt), "", "  ")
}

type yamlCodec struct{}

func (yamlCodec) ID() string        { return "yaml" }
func (yamlCodec) Extension() string { return ".yaml" }
func (yamlCodec) Export(in Input) ([]byte, error) {
	return yaml.Marshal(toStructured(in.Prompt))
}
