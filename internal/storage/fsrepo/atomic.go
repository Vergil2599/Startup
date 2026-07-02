package fsrepo

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic escribe datos de forma atómica: archivo temporal en el mismo
// directorio + fsync + rename. Un crash a mitad de escritura nunca deja el
// archivo destino corrupto.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".pes-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op si el rename tuvo éxito
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// fsync del directorio para que el rename sea durable (best effort en
	// sistemas que no lo soportan, p. ej. Windows).
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
