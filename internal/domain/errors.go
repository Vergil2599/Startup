package domain

import "errors"

// Errores de dominio tipados. La infraestructura los envuelve, nunca los redefine.
var (
	ErrEmptyTitle        = errors.New("el prompt necesita un título")
	ErrInvalidBlockType  = errors.New("tipo de bloque vacío o inválido")
	ErrDuplicateBlock    = errors.New("el prompt contiene bloques duplicados del mismo tipo")
	ErrNotFound          = errors.New("no encontrado")
	ErrUnresolvedVars    = errors.New("variables sin resolver en modo estricto")
	ErrTemplateCycle     = errors.New("ciclo detectado en la herencia de plantillas")
	ErrInvalidVarName    = errors.New("nombre de variable inválido (usa snake_case)")
	ErrConflict          = errors.New("el archivo cambió externamente (conflicto)")
	ErrInvalidFileFormat = errors.New("formato de archivo de prompt inválido")
)
