// Package sqlindex implementa el índice de búsqueda de PES sobre SQLite FTS5.
// El índice es DERIVADO y DESECHABLE: se reconstruye por completo desde el
// workspace; jamás es fuente de verdad.
package sqlindex

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS prompts (
  rowid INTEGER PRIMARY KEY,
  id TEXT UNIQUE NOT NULL,
  path TEXT UNIQUE NOT NULL,
  title TEXT NOT NULL,
  description TEXT DEFAULT '',
  category TEXT DEFAULT '',
  favorite INTEGER DEFAULT 0,
  content_hash TEXT NOT NULL,
  score INTEGER,
  updated_at TEXT,
  mtime INTEGER
);
CREATE TABLE IF NOT EXISTS tags (
  prompt_id TEXT NOT NULL,
  tag TEXT NOT NULL,
  PRIMARY KEY (prompt_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts USING fts5(
  title, description, body, tags,
  tokenize='unicode61 remove_diacritics 2'
);
`

// Index es el índice de búsqueda local.
type Index struct {
	db *sql.DB
}

// Open abre (o crea) el índice en la ruta dada. ":memory:" para tests.
func Open(path string) (*Index, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// El índice es un recurso local de un solo proceso.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;"); err != nil {
		db.Close()
		return nil, err
	}
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		db.Close()
		return nil, err
	}
	if v != 0 && v != schemaVersion {
		// Esquema desconocido: el índice es desechable, se reconstruye.
		if err := wipe(db); err != nil {
			db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version=%d", schemaVersion)); err != nil {
		db.Close()
		return nil, err
	}
	return &Index{db: db}, nil
}

func wipe(db *sql.DB) error {
	_, err := db.Exec(`
		DROP TABLE IF EXISTS prompts;
		DROP TABLE IF EXISTS tags;
		DROP TABLE IF EXISTS prompts_fts;
		PRAGMA user_version=0;`)
	return err
}

// Close cierra la conexión.
func (ix *Index) Close() error { return ix.db.Close() }

// Upsert indexa (o re-indexa) un prompt.
func (ix *Index) Upsert(e *fsrepo.Entry) error {
	p := e.Prompt
	tx, err := ix.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Si la ruta está ocupada por OTRO id (archivo reemplazado externamente,
	// p. ej. git checkout), purgar la fila vieja para no violar UNIQUE(path).
	var oldID string
	var oldRowid int64
	if err := tx.QueryRow("SELECT rowid, id FROM prompts WHERE path=?", e.Path).Scan(&oldRowid, &oldID); err == nil && oldID != string(p.ID) {
		for _, q := range []string{"DELETE FROM prompts_fts WHERE rowid=?", "DELETE FROM prompts WHERE rowid=?"} {
			if _, derr := tx.Exec(q, oldRowid); derr != nil {
				return derr
			}
		}
		if _, derr := tx.Exec("DELETE FROM tags WHERE prompt_id=?", oldID); derr != nil {
			return derr
		}
	}

	var rowid int64
	err = tx.QueryRow("SELECT rowid FROM prompts WHERE id=?", string(p.ID)).Scan(&rowid)
	switch err {
	case nil:
		if _, err := tx.Exec(`UPDATE prompts SET path=?, title=?, description=?, category=?,
			favorite=?, content_hash=?, updated_at=?, mtime=? WHERE rowid=?`,
			e.Path, p.Title, p.Description, p.Category, boolInt(p.Favorite),
			e.ContentHash, p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"), e.MTime, rowid); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM prompts_fts WHERE rowid=?", rowid); err != nil {
			return err
		}
	case sql.ErrNoRows:
		res, ierr := tx.Exec(`INSERT INTO prompts (id, path, title, description, category,
			favorite, content_hash, updated_at, mtime) VALUES (?,?,?,?,?,?,?,?,?)`,
			string(p.ID), e.Path, p.Title, p.Description, p.Category, boolInt(p.Favorite),
			e.ContentHash, p.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"), e.MTime)
		if ierr != nil {
			return ierr
		}
		rowid, _ = res.LastInsertId()
	default:
		return err
	}

	if _, err := tx.Exec("DELETE FROM tags WHERE prompt_id=?", string(p.ID)); err != nil {
		return err
	}
	for _, tag := range p.Tags {
		if _, err := tx.Exec("INSERT INTO tags (prompt_id, tag) VALUES (?,?)", string(p.ID), tag); err != nil {
			return err
		}
	}

	var body strings.Builder
	for _, b := range p.Blocks {
		body.WriteString(b.Content)
		body.WriteString("\n")
	}
	if _, err := tx.Exec("INSERT INTO prompts_fts (rowid, title, description, body, tags) VALUES (?,?,?,?,?)",
		rowid, p.Title, p.Description, body.String(), strings.Join(p.Tags, " ")); err != nil {
		return err
	}
	return tx.Commit()
}

// Remove elimina un prompt del índice por ruta.
func (ix *Index) Remove(relPath string) error {
	tx, err := ix.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var rowid int64
	var id string
	err = tx.QueryRow("SELECT rowid, id FROM prompts WHERE path=?", relPath).Scan(&rowid, &id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	for _, q := range []string{
		"DELETE FROM prompts_fts WHERE rowid=?",
		"DELETE FROM prompts WHERE rowid=?",
	} {
		if _, err := tx.Exec(q, rowid); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM tags WHERE prompt_id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// SetScore guarda la última puntuación de validación.
func (ix *Index) SetScore(id domain.ID, score int) error {
	_, err := ix.db.Exec("UPDATE prompts SET score=? WHERE id=?", score, string(id))
	return err
}

// Hit es un resultado de búsqueda o listado.
type Hit struct {
	ID       domain.ID
	Path     string
	Title    string
	Category string
	Favorite bool
	Score    *int
	Tags     []string
}

// Search ejecuta una búsqueda full-text con prefijos (implicit AND por término).
func (ix *Index) Search(query string, limit int) ([]Hit, error) {
	q := buildFTSQuery(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := ix.db.Query(`
		SELECT p.id, p.path, p.title, p.category, p.favorite, p.score
		FROM prompts_fts f JOIN prompts p ON p.rowid = f.rowid
		WHERE prompts_fts MATCH ? ORDER BY rank LIMIT ?`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return ix.scanHits(rows)
}

// ListFilter filtra el listado de la biblioteca.
type ListFilter struct {
	Tag       string
	Category  string
	Favorites bool
}

// List devuelve los prompts del índice según el filtro, ordenados por título.
func (ix *Index) List(f ListFilter) ([]Hit, error) {
	var where []string
	var args []any
	if f.Tag != "" {
		where = append(where, "p.id IN (SELECT prompt_id FROM tags WHERE tag=?)")
		args = append(args, strings.ToLower(f.Tag))
	}
	if f.Category != "" {
		where = append(where, "p.category=?")
		args = append(args, f.Category)
	}
	if f.Favorites {
		where = append(where, "p.favorite=1")
	}
	q := "SELECT p.id, p.path, p.title, p.category, p.favorite, p.score FROM prompts p"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY p.title COLLATE NOCASE"
	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return ix.scanHits(rows)
}

// Stale devuelve el mtime y hash indexados de una ruta (para reindexado
// incremental); ok=false si la ruta no está indexada.
func (ix *Index) Stale(relPath string) (mtime int64, hash string, ok bool, err error) {
	err = ix.db.QueryRow("SELECT mtime, content_hash FROM prompts WHERE path=?", relPath).Scan(&mtime, &hash)
	if err == sql.ErrNoRows {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	return mtime, hash, true, nil
}

// Paths devuelve todas las rutas indexadas (para detectar borrados externos).
func (ix *Index) Paths() (map[string]bool, error) {
	rows, err := ix.db.Query("SELECT path FROM prompts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = true
	}
	return out, rows.Err()
}

// Count devuelve el número de prompts indexados.
func (ix *Index) Count() (int, error) {
	var n int
	err := ix.db.QueryRow("SELECT COUNT(*) FROM prompts").Scan(&n)
	return n, err
}

func (ix *Index) scanHits(rows *sql.Rows) ([]Hit, error) {
	var hits []Hit
	for rows.Next() {
		var h Hit
		var fav int
		var score sql.NullInt64
		if err := rows.Scan((*string)(&h.ID), &h.Path, &h.Title, &h.Category, &fav, &score); err != nil {
			return nil, err
		}
		h.Favorite = fav == 1
		if score.Valid {
			s := int(score.Int64)
			h.Score = &s
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range hits {
		trows, err := ix.db.Query("SELECT tag FROM tags WHERE prompt_id=? ORDER BY tag", string(hits[i].ID))
		if err != nil {
			return nil, err
		}
		for trows.Next() {
			var t string
			if err := trows.Scan(&t); err != nil {
				trows.Close()
				return nil, err
			}
			hits[i].Tags = append(hits[i].Tags, t)
		}
		trows.Close()
	}
	return hits, nil
}

// buildFTSQuery convierte texto libre del usuario en una consulta FTS5 segura:
// cada término se convierte en frase con prefijo ("term"*), unidos por AND.
func buildFTSQuery(input string) string {
	fields := strings.Fields(input)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, "")
		if f == "" {
			continue
		}
		terms = append(terms, `"`+f+`"*`)
	}
	return strings.Join(terms, " ")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
