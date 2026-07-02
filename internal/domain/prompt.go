// Package domain contiene las entidades puras del Prompt Engineering Studio.
// Regla de arquitectura: este paquete no importa nada del proyecto.
package domain

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// SchemaVersion es la versión actual del esquema de archivo de prompt (campo `pes`).
const SchemaVersion = 1

// ID identifica de forma estable a cualquier entidad (ULID).
type ID string

// NewID genera un ULID nuevo.
func NewID() ID {
	return ID(ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String())
}

// BlockType es el tipo de una sección del prompt.
type BlockType string

// Tipos de bloque incorporados. Los plugins podrán registrar tipos adicionales.
const (
	BlockRole              BlockType = "role"
	BlockObjective         BlockType = "objective"
	BlockContext           BlockType = "context"
	BlockBackground        BlockType = "background"
	BlockConstraints       BlockType = "constraints"
	BlockRules             BlockType = "rules"
	BlockWorkflow          BlockType = "workflow"
	BlockInput             BlockType = "input"
	BlockOutput            BlockType = "output"
	BlockExamples          BlockType = "examples"
	BlockFailureConditions BlockType = "failure_conditions"
	BlockSuccessCriteria   BlockType = "success_criteria"
	BlockNotes             BlockType = "notes"
)

// BuiltinBlockTypes define el orden canónico de los bloques incorporados.
var BuiltinBlockTypes = []BlockType{
	BlockRole, BlockObjective, BlockContext, BlockBackground,
	BlockConstraints, BlockRules, BlockWorkflow, BlockInput, BlockOutput,
	BlockExamples, BlockFailureConditions, BlockSuccessCriteria, BlockNotes,
}

// IsBuiltinBlockType indica si t es un tipo de bloque incorporado.
func IsBuiltinBlockType(t BlockType) bool {
	for _, b := range BuiltinBlockTypes {
		if b == t {
			return true
		}
	}
	return false
}

// Block es una sección tipada de un prompt. Desactivar un bloque conserva su
// contenido pero lo excluye del render y de la validación de contenido.
type Block struct {
	Type    BlockType
	Enabled bool
	Content string
}

// IsEmpty indica si el bloque no tiene contenido efectivo.
func (b Block) IsEmpty() bool { return strings.TrimSpace(b.Content) == "" }

// Prompt es el artefacto central: un documento estructurado en bloques.
type Prompt struct {
	ID          ID
	Title       string
	Description string
	Tags        []string
	Category    string
	Favorite    bool
	TemplateID  ID     // plantilla de origen, si aplica
	Extends     ID     // solo para plantillas: plantilla padre
	Blocks      []Block
	Variables   map[string]string // ámbito prompt
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Schema      int
}

// Validate comprueba las invariantes de dominio de un prompt.
func (p *Prompt) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return ErrEmptyTitle
	}
	seen := map[BlockType]bool{}
	for _, b := range p.Blocks {
		if strings.TrimSpace(string(b.Type)) == "" {
			return ErrInvalidBlockType
		}
		if seen[b.Type] {
			return ErrDuplicateBlock
		}
		seen[b.Type] = true
	}
	return nil
}

// EnabledBlocks devuelve los bloques activos en orden.
func (p *Prompt) EnabledBlocks() []Block {
	out := make([]Block, 0, len(p.Blocks))
	for _, b := range p.Blocks {
		if b.Enabled {
			out = append(out, b)
		}
	}
	return out
}

// Block devuelve el bloque del tipo dado y si existe.
func (p *Prompt) Block(t BlockType) (Block, bool) {
	for _, b := range p.Blocks {
		if b.Type == t {
			return b, true
		}
	}
	return Block{}, false
}

// SetBlock reemplaza (o añade al final) el bloque del tipo dado.
func (p *Prompt) SetBlock(nb Block) {
	for i, b := range p.Blocks {
		if b.Type == nb.Type {
			p.Blocks[i] = nb
			return
		}
	}
	p.Blocks = append(p.Blocks, nb)
}

// NormalizeTags pone las etiquetas en minúsculas, sin espacios y sin duplicados.
func NormalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
