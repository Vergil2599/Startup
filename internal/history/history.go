// Package history implementa el historial de versiones de PES: snapshots
// inmutables content-addressed (SHA-256) bajo .pes/history/, con log por
// prompt y restauración que nunca reescribe la historia.
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// Store gestiona snapshots dentro de un workspace.
type Store struct {
	ws *fsrepo.Workspace
}

// New crea el store de historial del workspace.
func New(ws *fsrepo.Workspace) *Store { return &Store{ws: ws} }

func (s *Store) objectsDir() string {
	return filepath.Join(s.ws.Root, fsrepo.DirMeta, "history", "objects")
}

func (s *Store) logPath(id domain.ID) string {
	return filepath.Join(s.ws.Root, fsrepo.DirMeta, "history", string(id)+".jsonl")
}

type logEntry struct {
	ID      string    `json:"id"`
	Seq     int       `json:"seq"`
	Author  string    `json:"author,omitempty"`
	Comment string    `json:"comment,omitempty"`
	Hash    string    `json:"hash"`
	Created time.Time `json:"created"`
}

// Snapshot guarda una versión del estado actual en disco del prompt.
// Deduplica: si la última versión tiene el mismo hash, no crea otra.
func (s *Store) Snapshot(relPath, author, comment string) (domain.Version, error) {
	data, err := os.ReadFile(filepath.Join(s.ws.Root, relPath))
	if err != nil {
		return domain.Version{}, err
	}
	p, err := fsrepo.Parse(data)
	if err != nil {
		return domain.Version{}, err
	}
	hash := fsrepo.HashBytes(data)

	versions, err := s.List(p.ID)
	if err != nil {
		return domain.Version{}, err
	}
	if len(versions) > 0 && versions[len(versions)-1].ContentHash == hash {
		return versions[len(versions)-1], nil // sin cambios: no duplicar
	}

	obj := filepath.Join(s.objectsDir(), hash)
	if _, err := os.Stat(obj); os.IsNotExist(err) {
		if werr := fsrepo.WriteFileAtomic(obj, data); werr != nil {
			return domain.Version{}, werr
		}
	}

	v := domain.Version{
		ID: domain.NewID(), PromptID: p.ID, Seq: len(versions) + 1,
		Author: author, Comment: comment, ContentHash: hash,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	entry, _ := json.Marshal(logEntry{
		ID: string(v.ID), Seq: v.Seq, Author: v.Author, Comment: v.Comment,
		Hash: v.ContentHash, Created: v.CreatedAt,
	})
	if err := appendLine(s.logPath(p.ID), entry); err != nil {
		return domain.Version{}, err
	}
	return v, nil
}

// List devuelve las versiones de un prompt en orden cronológico.
func (s *Store) List(id domain.ID) ([]domain.Version, error) {
	f, err := os.Open(s.logPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []domain.Version
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e logEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue // línea corrupta: se ignora, el log sigue siendo usable
		}
		out = append(out, domain.Version{
			ID: domain.ID(e.ID), PromptID: id, Seq: e.Seq, Author: e.Author,
			Comment: e.Comment, ContentHash: e.Hash, CreatedAt: e.Created,
		})
	}
	return out, sc.Err()
}

// Load recupera el contenido serializado de una versión.
func (s *Store) Load(hash string) (*domain.Prompt, []byte, error) {
	data, err := os.ReadFile(filepath.Join(s.objectsDir(), hash))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, err
	}
	p, err := fsrepo.Parse(data)
	return p, data, err
}

// Restore vuelve a una versión anterior. Antes de restaurar toma un snapshot
// del estado actual, y la restauración se registra como una versión NUEVA:
// la historia nunca se reescribe.
func (s *Store) Restore(relPath string, versionRef string) (domain.Version, error) {
	cur, err := s.ws.LoadPrompt(relPath)
	if err != nil {
		return domain.Version{}, err
	}
	versions, err := s.List(cur.Prompt.ID)
	if err != nil {
		return domain.Version{}, err
	}
	var target *domain.Version
	for i := range versions {
		if string(versions[i].ID) == versionRef || fmt.Sprint(versions[i].Seq) == versionRef {
			target = &versions[i]
		}
	}
	if target == nil {
		return domain.Version{}, fmt.Errorf("%w: versión %q", domain.ErrNotFound, versionRef)
	}
	if _, err := s.Snapshot(relPath, "", "auto: antes de restaurar v"+fmt.Sprint(target.Seq)); err != nil {
		return domain.Version{}, err
	}
	_, data, err := s.Load(target.ContentHash)
	if err != nil {
		return domain.Version{}, err
	}
	if err := fsrepo.WriteFileAtomic(filepath.Join(s.ws.Root, relPath), data); err != nil {
		return domain.Version{}, err
	}
	return s.Snapshot(relPath, "", "restaurado desde v"+fmt.Sprint(target.Seq))
}

func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
