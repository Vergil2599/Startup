package domain

import "time"

// Version es un snapshot inmutable de un prompt en un momento dado.
type Version struct {
	ID          ID
	PromptID    ID
	Seq         int
	Author      string
	Comment     string
	ContentHash string // SHA-256 del archivo serializado
	CreatedAt   time.Time
}

// DiffKind clasifica un cambio estructural entre dos versiones.
type DiffKind string

const (
	DiffAdded    DiffKind = "added"
	DiffRemoved  DiffKind = "removed"
	DiffModified DiffKind = "modified"
	DiffToggled  DiffKind = "toggled" // enabled cambió, contenido no
)

// BlockDiff describe el cambio de un bloque entre dos prompts.
type BlockDiff struct {
	Type BlockType
	Kind DiffKind
}

// Diff es el resultado estructural de comparar dos prompts.
type Diff struct {
	Blocks     []BlockDiff
	WordsA     int
	WordsB     int
	TitleDiff  bool
	Equal      bool
}
