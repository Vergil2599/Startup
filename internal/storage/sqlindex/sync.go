package sqlindex

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// SyncStats resume un reindexado.
type SyncStats struct {
	Indexed int // nuevos o cambiados
	Removed int // borrados externamente
	Skipped int // sin cambios (mtime o hash iguales)
	Errors  int // archivos ilegibles
}

// Sync reindexa incrementalmente el workspace. Los archivos cuyo mtime no ha
// cambiado se saltan con un simple stat (sin leerlos ni parsearlos); solo los
// nuevos o modificados se cargan, y de esos, los que conservan el mismo hash
// tampoco se re-indexan.
func (ix *Index) Sync(ws *fsrepo.Workspace) (SyncStats, error) {
	var stats SyncStats
	indexed, err := ix.Paths()
	if err != nil {
		return stats, err
	}
	seen := map[string]bool{}
	root := filepath.Join(ws.Root, fsrepo.DirPrompts)
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if os.IsNotExist(werr) {
				return nil // el directorio puede no existir aún
			}
			return werr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(ws.Root, path)
		seen[rel] = true

		info, ierr := d.Info()
		if ierr != nil {
			stats.Errors++
			return nil
		}
		idxMtime, idxHash, ok, serr := ix.Stale(rel)
		if serr == nil && ok && idxMtime == info.ModTime().UnixNano() {
			stats.Skipped++ // mtime intacto: ni siquiera se lee el archivo
			return nil
		}
		e, lerr := ws.LoadPrompt(rel)
		if lerr != nil {
			stats.Errors++
			return nil
		}
		if ok && idxHash == e.ContentHash {
			// mtime cambió (p. ej. touch) pero el contenido es idéntico:
			// refrescar solo el mtime para no volver a leerlo la próxima vez.
			if _, uerr := ix.db.Exec("UPDATE prompts SET mtime=? WHERE path=?", e.MTime, rel); uerr == nil {
				stats.Skipped++
				return nil
			}
		}
		if uerr := ix.Upsert(e); uerr != nil {
			stats.Errors++
			return nil
		}
		stats.Indexed++
		return nil
	})
	if walkErr != nil {
		return stats, walkErr
	}
	for path := range indexed {
		if !seen[path] {
			if rerr := ix.Remove(path); rerr != nil {
				stats.Errors++
				continue
			}
			stats.Removed++
		}
	}
	return stats, nil
}

// Rebuild borra el índice y lo reconstruye desde cero.
func (ix *Index) Rebuild(ws *fsrepo.Workspace) (SyncStats, error) {
	if err := wipe(ix.db); err != nil {
		return SyncStats{}, err
	}
	if _, err := ix.db.Exec(schema); err != nil {
		return SyncStats{}, err
	}
	return ix.Sync(ws)
}
