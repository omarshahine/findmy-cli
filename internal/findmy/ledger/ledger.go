// Package ledger stores Find My observations in an append-only SQLite history.
//
// Schema:
//
//	CREATE TABLE observations (
//	    id INTEGER PRIMARY KEY,
//	    ts TEXT NOT NULL,
//	    kind TEXT NOT NULL,
//	    name TEXT NOT NULL,
//	    location TEXT,
//	    staleness TEXT,
//	    distance TEXT,
//	    battery TEXT,
//	    raw_json TEXT NOT NULL
//	);
//
// The raw_json column preserves the full parsed record so future structured
// fields can be recovered without changing this minimal v1 schema.
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS observations (
	id INTEGER PRIMARY KEY,
	ts TEXT NOT NULL,
	kind TEXT NOT NULL,
	name TEXT NOT NULL,
	location TEXT,
	staleness TEXT,
	distance TEXT,
	battery TEXT,
	raw_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_observations_name_ts ON observations(name, ts);
CREATE INDEX IF NOT EXISTS idx_observations_kind_ts ON observations(kind, ts);
`

type Ledger struct {
	db *sql.DB
}

type Observation struct {
	Ts        time.Time       `json:"ts"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Location  string          `json:"location,omitempty"`
	Staleness string          `json:"staleness,omitempty"`
	Distance  string          `json:"distance,omitempty"`
	Battery   string          `json:"battery,omitempty"`
	Raw       json.RawMessage `json:"raw_json"`
}

type QueryOptions struct {
	Name  string
	Kind  string
	Since time.Time
	Until time.Time
	Limit int
}

func Open(path string) (*Ledger, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := mkdirForDB(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite ledger: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize sqlite ledger: %w", err)
	}
	return &Ledger{db: db}, nil
}

func mkdirForDB(path string) error {
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create ledger directory: %w", err)
	}
	return nil
}

func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

func (l *Ledger) Append(ctx context.Context, obs []Observation) error {
	if len(obs) == 0 {
		return nil
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ledger append: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO observations (ts, kind, name, location, staleness, distance, battery, raw_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("prepare ledger append: %w", err)
	}
	defer stmt.Close()

	for _, o := range obs {
		raw := o.Raw
		if len(raw) == 0 {
			raw = json.RawMessage("{}")
		}
		if _, err := stmt.ExecContext(
			ctx,
			o.Ts.UTC().Format(time.RFC3339Nano),
			o.Kind,
			o.Name,
			o.Location,
			o.Staleness,
			o.Distance,
			o.Battery,
			string(raw),
		); err != nil {
			return fmt.Errorf("append ledger observation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ledger append: %w", err)
	}
	return nil
}

func (l *Ledger) Query(ctx context.Context, opts QueryOptions) ([]Observation, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	rows, err := l.query(ctx, opts, true)
	if err != nil {
		return nil, err
	}
	if opts.Name == "" || len(rows) > 0 {
		return rows, nil
	}
	return l.query(ctx, opts, false)
}

func (l *Ledger) query(ctx context.Context, opts QueryOptions, exactName bool) ([]Observation, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	where := []string{"1=1"}
	args := []any{}
	if opts.Name != "" {
		if exactName {
			where = append(where, "lower(name) = lower(?)")
			args = append(args, opts.Name)
		} else {
			where = append(where, "lower(name) LIKE ?")
			args = append(args, "%"+strings.ToLower(opts.Name)+"%")
		}
	}
	if opts.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, opts.Kind)
	}
	if !opts.Since.IsZero() {
		where = append(where, "ts >= ?")
		args = append(args, opts.Since.UTC().Format(time.RFC3339Nano))
	}
	if !opts.Until.IsZero() {
		where = append(where, "ts <= ?")
		args = append(args, opts.Until.UTC().Format(time.RFC3339Nano))
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
SELECT ts, kind, name, location, staleness, distance, battery, raw_json
FROM (
	SELECT ts, kind, name, location, staleness, distance, battery, raw_json
	FROM observations
	WHERE %s
	ORDER BY ts DESC
	LIMIT ?
)
ORDER BY ts ASC
`, strings.Join(where, " AND "))

	sqlRows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ledger observations: %w", err)
	}
	defer sqlRows.Close()

	var out []Observation
	for sqlRows.Next() {
		var o Observation
		var ts, raw string
		if err := sqlRows.Scan(&ts, &o.Kind, &o.Name, &o.Location, &o.Staleness, &o.Distance, &o.Battery, &raw); err != nil {
			return nil, fmt.Errorf("scan ledger observation: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("parse ledger timestamp: %w", err)
		}
		o.Ts = parsed
		o.Raw = json.RawMessage(raw)
		out = append(out, o)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ledger observations: %w", err)
	}
	return out, nil
}

func DefaultPath() string {
	if env := os.Getenv("FINDMY_HISTORY_DB"); env != "" {
		return env
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "findmy-cli", "history.sqlite")
}
