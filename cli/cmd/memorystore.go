package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const memorySchema = `
CREATE TABLE IF NOT EXISTS memory (
  project    TEXT    NOT NULL,
  group_name TEXT    NOT NULL DEFAULT '',
  key        TEXT    NOT NULL,
  value      TEXT    NOT NULL,
  tags       TEXT    NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (project, key)
);

CREATE INDEX IF NOT EXISTS idx_memory_group ON memory(group_name);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
  key, value, tags,
  content='memory',
  content_rowid='rowid',
  tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS memory_ai AFTER INSERT ON memory BEGIN
  INSERT INTO memory_fts(rowid, key, value, tags)
  VALUES (new.rowid, new.key, new.value, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS memory_ad AFTER DELETE ON memory BEGIN
  INSERT INTO memory_fts(memory_fts, rowid, key, value, tags)
  VALUES ('delete', old.rowid, old.key, old.value, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS memory_au AFTER UPDATE ON memory BEGIN
  INSERT INTO memory_fts(memory_fts, rowid, key, value, tags)
  VALUES ('delete', old.rowid, old.key, old.value, old.tags);
  INSERT INTO memory_fts(rowid, key, value, tags)
  VALUES (new.rowid, new.key, new.value, new.tags);
END;
`

// MemoryEntry represents a single memory record.
type MemoryEntry struct {
	Project   string `json:"project"             yaml:"project"`
	Group     string `json:"group"               yaml:"group"`
	Key       string `json:"key"                 yaml:"key"`
	Value     string `json:"value"               yaml:"value"`
	Tags      string `json:"tags,omitempty"      yaml:"tags,omitempty"`
	CreatedAt int64  `json:"created_at"          yaml:"created_at"`
	UpdatedAt int64  `json:"updated_at"          yaml:"updated_at"`
}

// memoryDB is the shared open database connection.
var memoryDB *sql.DB

// openMemoryDB opens (or creates) the memory SQLite database.
func openMemoryDB() (*sql.DB, error) {
	if memoryDB != nil {
		return memoryDB, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".sdt")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("cannot create ~/.sdt: %w", err)
	}
	dbPath := filepath.Join(dir, "memory.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open memory database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(memorySchema); err != nil {
		return nil, fmt.Errorf("cannot initialise memory schema: %w", err)
	}
	memoryDB = db
	return db, nil
}

func memorySet(project, group, key, value, tags string) error {
	db, err := openMemoryDB()
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = db.Exec(`
		INSERT INTO memory(project, group_name, key, value, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project, key) DO UPDATE SET
		  group_name=excluded.group_name,
		  value=excluded.value,
		  tags=excluded.tags,
		  updated_at=excluded.updated_at
	`, project, group, key, value, tags, now, now)
	return err
}

func memoryGet(project, key string) (*MemoryEntry, error) {
	db, err := openMemoryDB()
	if err != nil {
		return nil, err
	}
	row := db.QueryRow(`
		SELECT project, group_name, key, value, tags, created_at, updated_at
		FROM memory WHERE project=? AND key=?
	`, project, key)
	var e MemoryEntry
	if err := row.Scan(&e.Project, &e.Group, &e.Key, &e.Value, &e.Tags, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func memoryList(project, group string) ([]MemoryEntry, error) {
	db, err := openMemoryDB()
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if group != "" && project == "" {
		rows, err = db.Query(`
			SELECT project, group_name, key, value, tags, created_at, updated_at
			FROM memory WHERE group_name=? ORDER BY project, key
		`, group)
	} else if project != "" && group != "" {
		rows, err = db.Query(`
			SELECT project, group_name, key, value, tags, created_at, updated_at
			FROM memory WHERE project=? AND group_name=? ORDER BY key
		`, project, group)
	} else {
		rows, err = db.Query(`
			SELECT project, group_name, key, value, tags, created_at, updated_at
			FROM memory WHERE project=? ORDER BY key
		`, project)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	return scanEntries(rows)
}

func memorySearch(query, project, group string, top int) ([]MemoryEntry, error) {
	db, err := openMemoryDB()
	if err != nil {
		return nil, err
	}
	limit := 20
	if top > 0 {
		limit = top
	}

	var rows *sql.Rows
	if project != "" {
		rows, err = db.Query(`
			SELECT m.project, m.group_name, m.key, m.value, m.tags, m.created_at, m.updated_at
			FROM memory m
			JOIN memory_fts f ON m.rowid = f.rowid
			WHERE memory_fts MATCH ? AND m.project=?
			ORDER BY bm25(memory_fts)
			LIMIT ?
		`, query, project, limit)
	} else if group != "" {
		rows, err = db.Query(`
			SELECT m.project, m.group_name, m.key, m.value, m.tags, m.created_at, m.updated_at
			FROM memory m
			JOIN memory_fts f ON m.rowid = f.rowid
			WHERE memory_fts MATCH ? AND m.group_name=?
			ORDER BY bm25(memory_fts)
			LIMIT ?
		`, query, group, limit)
	} else {
		rows, err = db.Query(`
			SELECT m.project, m.group_name, m.key, m.value, m.tags, m.created_at, m.updated_at
			FROM memory m
			JOIN memory_fts f ON m.rowid = f.rowid
			WHERE memory_fts MATCH ?
			ORDER BY bm25(memory_fts)
			LIMIT ?
		`, query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	return scanEntries(rows)
}

func memoryDelete(project, key string) error {
	db, err := openMemoryDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM memory WHERE project=? AND key=?`, project, key)
	return err
}

func memoryDeleteAll(project string) error {
	db, err := openMemoryDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM memory WHERE project=?`, project)
	return err
}

func memoryProjects() ([]string, error) {
	db, err := openMemoryDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT DISTINCT project FROM memory ORDER BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func memoryGroups() ([]string, error) {
	db, err := openMemoryDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT DISTINCT group_name FROM memory WHERE group_name != '' ORDER BY group_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func memoryExport(project string) ([]MemoryEntry, error) {
	db, err := openMemoryDB()
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if project != "" {
		rows, err = db.Query(`
			SELECT project, group_name, key, value, tags, created_at, updated_at
			FROM memory WHERE project=? ORDER BY key
		`, project)
	} else {
		rows, err = db.Query(`
			SELECT project, group_name, key, value, tags, created_at, updated_at
			FROM memory ORDER BY project, key
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	return scanEntries(rows)
}

func memoryImport(entries []MemoryEntry) error {
	for _, e := range entries {
		if err := memorySet(e.Project, e.Group, e.Key, e.Value, e.Tags); err != nil {
			return fmt.Errorf("import entry %q/%q: %w", e.Project, e.Key, err)
		}
	}
	return nil
}

func scanEntries(rows *sql.Rows) ([]MemoryEntry, error) {
	var out []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.Project, &e.Group, &e.Key, &e.Value, &e.Tags, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []MemoryEntry{}
	}
	return out, rows.Err()
}

// normalizeTags trims spaces around comma-separated tag values.
func normalizeTags(tags string) string {
	parts := strings.Split(tags, ",")
	normalized := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			normalized = append(normalized, p)
		}
	}
	return strings.Join(normalized, ",")
}
