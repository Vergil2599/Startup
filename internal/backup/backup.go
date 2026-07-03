// Package backup implementa copias de seguridad rotadas del workspace:
// tar.gz de las carpetas de contenido (nunca de .pes/, que es derivado salvo
// history y runs, incluidos explícitamente), con cifrado age opcional.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// Dirs incluidas en el backup (relativas al workspace).
var includeDirs = []string{
	fsrepo.DirPrompts, fsrepo.DirTemplates, fsrepo.DirVariables, "compositions",
	filepath.Join(fsrepo.DirMeta, "history"),
}

func backupsDir(ws *fsrepo.Workspace) string {
	return filepath.Join(ws.Root, fsrepo.DirMeta, "backups")
}

// Create genera un backup nuevo. Si passphrase no es vacía, el archivo se
// cifra con age (scrypt) y lleva extensión .age.
func Create(ws *fsrepo.Workspace, passphrase string) (string, error) {
	name := "pes-backup-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	if passphrase != "" {
		name += ".age"
	}
	dst := filepath.Join(backupsDir(ws), name)
	if err := os.MkdirAll(backupsDir(ws), 0o755); err != nil {
		return "", err
	}
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	var sink io.WriteCloser = f
	var ageW io.WriteCloser
	if passphrase != "" {
		recipient, rerr := age.NewScryptRecipient(passphrase)
		if rerr != nil {
			f.Close()
			return "", rerr
		}
		ageW, err = age.Encrypt(f, recipient)
		if err != nil {
			f.Close()
			return "", err
		}
		sink = ageW
	}
	gz := gzip.NewWriter(sink)
	tw := tar.NewWriter(gz)

	writeErr := func() error {
		for _, dir := range includeDirs {
			root := filepath.Join(ws.Root, dir)
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
				if werr != nil {
					if os.IsNotExist(werr) {
						return nil
					}
					return werr
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(ws.Root, path)
				info, ierr := d.Info()
				if ierr != nil {
					return ierr
				}
				hdr := &tar.Header{
					Name: filepath.ToSlash(rel), Mode: 0o644,
					Size: info.Size(), ModTime: info.ModTime(),
				}
				if herr := tw.WriteHeader(hdr); herr != nil {
					return herr
				}
				src, oerr := os.Open(path)
				if oerr != nil {
					return oerr
				}
				defer src.Close()
				_, cerr := io.Copy(tw, src)
				return cerr
			})
			if err != nil {
				return err
			}
		}
		return nil
	}()

	if err := tw.Close(); err != nil && writeErr == nil {
		writeErr = err
	}
	if err := gz.Close(); err != nil && writeErr == nil {
		writeErr = err
	}
	if ageW != nil {
		if err := ageW.Close(); err != nil && writeErr == nil {
			writeErr = err
		}
	}
	if err := f.Sync(); err != nil && writeErr == nil {
		writeErr = err
	}
	if err := f.Close(); err != nil && writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		return "", writeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// List devuelve los backups existentes (más recientes primero).
func List(ws *fsrepo.Workspace) ([]string, error) {
	entries, err := os.ReadDir(backupsDir(ws))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "pes-backup-") {
			out = append(out, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out))) // el timestamp ordena
	return out, nil
}

// Rotate conserva como máximo keep backups, eliminando los más antiguos.
func Rotate(ws *fsrepo.Workspace, keep int) (removed int, err error) {
	if keep < 1 {
		keep = 1
	}
	names, err := List(ws)
	if err != nil {
		return 0, err
	}
	for _, name := range names[minInt(keep, len(names)):] {
		if rerr := os.Remove(filepath.Join(backupsDir(ws), name)); rerr != nil {
			return removed, rerr
		}
		removed++
	}
	return removed, nil
}

// Entries abre un backup (descifrándolo si procede) y devuelve las rutas contenidas.
func Entries(path, passphrase string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var src io.Reader = f
	if strings.HasSuffix(path, ".age") {
		if passphrase == "" {
			return nil, fmt.Errorf("el backup está cifrado: se requiere passphrase")
		}
		identity, ierr := age.NewScryptIdentity(passphrase)
		if ierr != nil {
			return nil, ierr
		}
		src, err = age.Decrypt(f, identity)
		if err != nil {
			return nil, fmt.Errorf("no se pudo descifrar (¿passphrase incorrecta?): %w", err)
		}
	}
	gz, err := gzip.NewReader(src)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		names = append(names, hdr.Name)
	}
	sort.Strings(names)
	return names, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
