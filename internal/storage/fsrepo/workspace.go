package fsrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

// Workspace es la carpeta del usuario que actúa como fuente de verdad.
type Workspace struct {
	Root string
}

// Layout del workspace.
const (
	DirPrompts   = "prompts"
	DirTemplates = "templates"
	DirVariables = "variables"
	DirMeta      = ".pes"
)

// Open valida que root sea un workspace de PES (contiene .pes/).
func Open(root string) (*Workspace, error) {
	info, err := os.Stat(filepath.Join(root, DirMeta))
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%q no es un workspace de PES (ejecuta `pes init`)", root)
	}
	return &Workspace{Root: root}, nil
}

// Init crea la estructura de un workspace nuevo (idempotente).
func Init(root string) (*Workspace, error) {
	for _, d := range []string{DirPrompts, DirTemplates, DirVariables, DirMeta} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	globalVars := filepath.Join(root, DirVariables, "global.yaml")
	if _, err := os.Stat(globalVars); os.IsNotExist(err) {
		if err := WriteFileAtomic(globalVars, []byte("# Variables globales: nombre: valor\n{}\n")); err != nil {
			return nil, err
		}
	}
	return &Workspace{Root: root}, nil
}

// Entry es un prompt localizado en disco con su hash de contenido.
type Entry struct {
	Prompt      *domain.Prompt
	Path        string // relativo al root del workspace
	ContentHash string
	MTime       int64
}

// SavePrompt escribe un prompt. Si relPath es vacío se deriva de la carpeta
// por defecto y el título. Devuelve la ruta relativa y el hash del contenido.
func (w *Workspace) SavePrompt(p *domain.Prompt, relPath string) (string, string, error) {
	if err := p.Validate(); err != nil {
		return "", "", err
	}
	if relPath == "" {
		relPath = filepath.Join(DirPrompts, Slug(p.Title)+".md")
	}
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	data, err := Serialize(p)
	if err != nil {
		return "", "", err
	}
	if err := WriteFileAtomic(filepath.Join(w.Root, relPath), data); err != nil {
		return "", "", err
	}
	return relPath, HashBytes(data), nil
}

// LoadPrompt lee y parsea un prompt por su ruta relativa.
func (w *Workspace) LoadPrompt(relPath string) (*Entry, error) {
	abs := filepath.Join(w.Root, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	p, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relPath, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	return &Entry{Prompt: p, Path: relPath, ContentHash: HashBytes(data), MTime: info.ModTime().UnixNano()}, nil
}

// DeletePrompt elimina el archivo de un prompt.
func (w *Workspace) DeletePrompt(relPath string) error {
	err := os.Remove(filepath.Join(w.Root, relPath))
	if os.IsNotExist(err) {
		return domain.ErrNotFound
	}
	return err
}

// Walk recorre todos los prompts o plantillas del workspace. Los archivos
// ilegibles se reportan a onError y no interrumpen el recorrido.
func (w *Workspace) Walk(dir string, fn func(*Entry), onError func(path string, err error)) error {
	root := filepath.Join(w.Root, dir)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // el directorio puede no existir aún
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(w.Root, path)
		e, perr := w.LoadPrompt(rel)
		if perr != nil {
			if onError != nil {
				onError(rel, perr)
			}
			return nil
		}
		fn(e)
		return nil
	})
}

// FindByID localiza un prompt o plantilla por ID recorriendo el workspace.
// (El índice SQLite ofrece la vía rápida; esto es el fallback sin índice.)
func (w *Workspace) FindByID(id domain.ID) (*Entry, error) {
	var found *Entry
	for _, dir := range []string{DirPrompts, DirTemplates} {
		_ = w.Walk(dir, func(e *Entry) {
			if e.Prompt.ID == id && found == nil {
				found = e
			}
		}, nil)
		if found != nil {
			return found, nil
		}
	}
	return nil, domain.ErrNotFound
}

// LoadVariables carga las variables de un archivo YAML plano (nombre: valor).
func (w *Workspace) LoadVariables(name string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(w.Root, DirVariables, name+".yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	out := map[string]string{}
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("variables/%s.yaml: %w", name, err)
	}
	for k := range out {
		if !domain.ValidVariableName(k) {
			return nil, fmt.Errorf("%w: %q en variables/%s.yaml", domain.ErrInvalidVarName, k, name)
		}
	}
	return out, nil
}

// SaveVariables persiste un conjunto de variables YAML.
func (w *Workspace) SaveVariables(name string, vars map[string]string) error {
	for k := range vars {
		if !domain.ValidVariableName(k) {
			return fmt.Errorf("%w: %q", domain.ErrInvalidVarName, k)
		}
	}
	data, err := yaml.Marshal(vars)
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(w.Root, DirVariables, name+".yaml"), data)
}

// HashBytes devuelve el SHA-256 hex de un contenido.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug convierte un título en un nombre de archivo estable y legible.
func Slug(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	replacer := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u")
	s = replacer.Replace(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "prompt"
	}
	return s
}
