package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"quadratic/internal/browse"

	_ "modernc.org/sqlite"
)

func (s *Store) ensureArchive(ctx context.Context) error {
	if err := s.initArchiveSchema(); err != nil {
		return err
	}
	return s.importJSONBackups(ctx)
}

func (s *Store) PrepareArchive(ctx context.Context) error {
	return s.ensureArchive(ctx)
}

func (s *Store) initArchiveSchema() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS checkins (
			id TEXT PRIMARY KEY,
			created_at INTEGER NOT NULL,
			created_at_text TEXT NOT NULL,
			shout TEXT NOT NULL DEFAULT '',
			venue_name TEXT NOT NULL DEFAULT '',
			city TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT '',
			country TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT '',
			people_json TEXT NOT NULL DEFAULT '[]',
			photos_json TEXT NOT NULL DEFAULT '[]',
			has_photos INTEGER NOT NULL DEFAULT 0,
			searchable_text TEXT NOT NULL DEFAULT '',
			raw_json TEXT NOT NULL,
			source_file TEXT NOT NULL,
			source_mtime_ns INTEGER NOT NULL DEFAULT 0,
			imported_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_checkins_created_at ON checkins(created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_checkins_has_photos_created_at ON checkins(has_photos, created_at DESC)`,
	}
	for _, stmt := range schema {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init archive schema: %w", err)
		}
	}
	return nil
}

func (s *Store) importJSONBackups(ctx context.Context) error {
	entries, err := os.ReadDir(s.checkinsDir)
	if err != nil {
		return fmt.Errorf("read checkins dir: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin archive import: %w", err)
	}
	defer tx.Rollback()

	upsert, err := tx.PrepareContext(ctx, `INSERT INTO checkins (
		id, created_at, created_at_text, shout, venue_name, city, state, country, source, category,
		people_json, photos_json, has_photos, searchable_text, raw_json, source_file, source_mtime_ns, imported_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		created_at=excluded.created_at,
		created_at_text=excluded.created_at_text,
		shout=excluded.shout,
		venue_name=excluded.venue_name,
		city=excluded.city,
		state=excluded.state,
		country=excluded.country,
		source=excluded.source,
		category=excluded.category,
		people_json=excluded.people_json,
		photos_json=excluded.photos_json,
		has_photos=excluded.has_photos,
		searchable_text=excluded.searchable_text,
		raw_json=excluded.raw_json,
		source_file=excluded.source_file,
		source_mtime_ns=excluded.source_mtime_ns,
		imported_at=excluded.imported_at
	WHERE checkins.source_mtime_ns <> excluded.source_mtime_ns OR checkins.raw_json <> excluded.raw_json`)
	if err != nil {
		return fmt.Errorf("prepare archive upsert: %w", err)
	}
	defer upsert.Close()

	seenIDs := make([]string, 0, len(entries))
	now := time.Now().Unix()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		path := filepath.Join(s.checkinsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat checkin file %s: %w", entry.Name(), err)
		}

		rawJSON, payload, err := parseCheckinFile(path)
		if err != nil {
			return fmt.Errorf("parse checkin file %s: %w", entry.Name(), err)
		}

		peopleJSON, err := json.Marshal(payload.People)
		if err != nil {
			return fmt.Errorf("marshal people for %s: %w", payload.ID, err)
		}
		photosJSON, err := json.Marshal(payload.Photos)
		if err != nil {
			return fmt.Errorf("marshal photos for %s: %w", payload.ID, err)
		}

		if _, err := upsert.ExecContext(
			ctx,
			payload.ID,
			payload.CreatedAt,
			payload.Date,
			payload.Shout,
			payload.VenueName,
			payload.City,
			payload.State,
			payload.Country,
			payload.Source,
			payload.Category,
			string(peopleJSON),
			string(photosJSON),
			boolToInt(len(payload.Photos) > 0),
			payload.SearchableText(),
			string(rawJSON),
			path,
			info.ModTime().UnixNano(),
			now,
		); err != nil {
			return fmt.Errorf("upsert checkin %s: %w", payload.ID, err)
		}

		seenIDs = append(seenIDs, payload.ID)
	}

	if err := deleteMissingRows(ctx, tx, seenIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit archive import: %w", err)
	}
	return nil
}

func deleteMissingRows(ctx context.Context, tx *sql.Tx, ids []string) error {
	if len(ids) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM checkins`)
		return err
	}
	sort.Strings(ids)
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := `DELETE FROM checkins WHERE id NOT IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete missing checkins: %w", err)
	}
	return nil
}

func (s *Store) ArchiveMeta(ctx context.Context) (*browse.Meta, error) {
	if err := s.ensureArchive(ctx); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MIN(created_at_text), ''), COALESCE(MAX(created_at_text), '') FROM checkins`)
	var meta browse.Meta
	meta.DataDir = s.rootDir
	meta.DBPath = s.dbPath
	if err := row.Scan(&meta.Count, &meta.MinDate, &meta.MaxDate); err != nil {
		return nil, fmt.Errorf("load archive meta: %w", err)
	}
	return &meta, nil
}

func (s *Store) QueryCheckins(ctx context.Context, opts browse.Query) ([]browse.Summary, int, error) {
	if err := s.ensureArchive(ctx); err != nil {
		return nil, 0, err
	}
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 50
	}
	if opts.PageSize > 200 {
		opts.PageSize = 200
	}

	where, args := queryFilters(opts)
	countSQL := `SELECT COUNT(*) FROM checkins` + where
	var total int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count checkins: %w", err)
	}

	listSQL := `SELECT id, created_at, created_at_text, shout, venue_name, city, state, country, source, category, people_json, photos_json
		FROM checkins` + where + ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, opts.PageSize, (opts.Page-1)*opts.PageSize)

	rows, err := s.db.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query checkins: %w", err)
	}
	defer rows.Close()

	items := make([]browse.Summary, 0, opts.PageSize)
	for rows.Next() {
		item, err := scanSummary(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate checkins: %w", err)
	}
	return items, total, nil
}

func (s *Store) RandomCheckin(ctx context.Context, opts browse.Query) (*browse.Detail, error) {
	if err := s.ensureArchive(ctx); err != nil {
		return nil, err
	}
	where, args := queryFilters(opts)
	query := `SELECT id FROM checkins` + where + ` ORDER BY RANDOM() LIMIT 1`
	var id string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		if errorsIsNoRows(err) {
			return nil, fmt.Errorf("no matching checkins")
		}
		return nil, fmt.Errorf("select random checkin: %w", err)
	}
	return s.LoadCheckin(ctx, id)
}

func (s *Store) LoadCheckin(ctx context.Context, id string) (*browse.Detail, error) {
	if err := s.ensureArchive(ctx); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, created_at, created_at_text, shout, venue_name, city, state, country, source, category, people_json, photos_json, raw_json
		FROM checkins WHERE id = ?`, id)
	var (
		item      browse.Summary
		peopleRaw string
		photosRaw string
		rawJSON   string
	)
	if err := row.Scan(
		&item.ID,
		&item.CreatedAt,
		&item.Date,
		&item.Shout,
		&item.VenueName,
		&item.City,
		&item.State,
		&item.Country,
		&item.Source,
		&item.Category,
		&peopleRaw,
		&photosRaw,
		&rawJSON,
	); err != nil {
		if errorsIsNoRows(err) {
			return nil, fmt.Errorf("checkin %s not found", id)
		}
		return nil, fmt.Errorf("load checkin %s: %w", id, err)
	}
	_ = json.Unmarshal([]byte(peopleRaw), &item.People)
	_ = json.Unmarshal([]byte(photosRaw), &item.Photos)

	var raw map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return nil, fmt.Errorf("decode raw checkin %s: %w", id, err)
	}
	return &browse.Detail{
		Summary: item,
		Raw:     raw,
		Pretty:  rawJSON,
	}, nil
}

func scanSummary(scanner interface{ Scan(...any) error }) (browse.Summary, error) {
	var (
		item      browse.Summary
		peopleRaw string
		photosRaw string
	)
	if err := scanner.Scan(
		&item.ID,
		&item.CreatedAt,
		&item.Date,
		&item.Shout,
		&item.VenueName,
		&item.City,
		&item.State,
		&item.Country,
		&item.Source,
		&item.Category,
		&peopleRaw,
		&photosRaw,
	); err != nil {
		return browse.Summary{}, fmt.Errorf("scan summary: %w", err)
	}
	_ = json.Unmarshal([]byte(peopleRaw), &item.People)
	_ = json.Unmarshal([]byte(photosRaw), &item.Photos)
	return item, nil
}

func queryFilters(opts browse.Query) (string, []any) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if q := strings.TrimSpace(strings.ToLower(opts.Query)); q != "" {
		clauses = append(clauses, `searchable_text LIKE ?`)
		args = append(args, "%"+q+"%")
	}
	if value := normalizeDateStart(opts.StartDate); value != 0 {
		clauses = append(clauses, `created_at >= ?`)
		args = append(args, value)
	}
	if value := normalizeDateEnd(opts.EndDate); value != 0 {
		clauses = append(clauses, `created_at <= ?`)
		args = append(args, value)
	}
	if opts.HasPhotos {
		clauses = append(clauses, `has_photos = 1`)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return ` WHERE ` + strings.Join(clauses, ` AND `), args
}

func normalizeDateStart(value string) int64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func normalizeDateEnd(value string) int64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0
	}
	return t.Add(24*time.Hour - time.Second).Unix()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}
