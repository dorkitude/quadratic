package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"quadratic/internal/foursquare"
)

type checkinFetcher interface {
	FetchCheckins(ctx context.Context, limit, offset int) (*foursquare.CheckinsPage, error)
}

type Store struct {
	rootDir     string
	checkinsDir string
	dbPath      string
	db          *sql.DB
	statePath   string
}

type State struct {
	LastSyncAt    time.Time `json:"last_sync_at"`
	TotalCheckins int       `json:"total_checkins"`
}

type SyncResult struct {
	Stored       int       `json:"stored"`
	Skipped      int       `json:"skipped"`
	TotalRemote  int       `json:"total_remote"`
	FinishedAt   time.Time `json:"finished_at"`
	CheckinsPath string    `json:"checkins_path"`
}

type Summary struct {
	TokenPresent bool
	Stored       int
	LastSyncAt   time.Time
	DataDir      string
}

func New(root string) (*Store, error) {
	checkinsDir := filepath.Join(root, "checkins")
	if err := os.MkdirAll(checkinsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(root, "archive.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite archive: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Store{
		rootDir:     root,
		checkinsDir: checkinsDir,
		dbPath:      dbPath,
		db:          db,
		statePath:   filepath.Join(root, "state.json"),
	}, nil
}

func (s *Store) SyncCheckins(ctx context.Context, client checkinFetcher) (*SyncResult, error) {
	result := &SyncResult{CheckinsPath: s.checkinsDir}
	offset := 0
	limit := 250

	for {
		page, err := client.FetchCheckins(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		result.TotalRemote = page.Count
		if len(page.Items) == 0 {
			break
		}

		for _, item := range page.Items {
			written, err := s.writeCheckin(item)
			if err != nil {
				return nil, err
			}
			if written {
				result.Stored++
			} else {
				result.Skipped++
			}
		}

		offset += len(page.Items)
		if offset >= page.Count {
			break
		}
	}

	result.FinishedAt = time.Now().UTC()
	if err := s.ensureArchive(ctx); err != nil {
		return nil, err
	}
	if err := s.writeState(State{
		LastSyncAt:    result.FinishedAt,
		TotalCheckins: s.countArchiveRows(),
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Store) Summary() (*Summary, error) {
	state, err := s.readState()
	if err != nil {
		return nil, err
	}
	return &Summary{
		Stored:     s.countArchiveRows(),
		LastSyncAt: state.LastSyncAt,
		DataDir:    s.rootDir,
	}, nil
}

func (s *Store) ListCheckins() ([]string, error) {
	entries, err := os.ReadDir(s.checkinsDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(s.checkinsDir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func (s *Store) writeCheckin(checkin foursquare.Checkin) (bool, error) {
	if checkin.ID == "" {
		return false, fmt.Errorf("checkin missing id")
	}

	path := filepath.Join(s.checkinsDir, checkin.ID+".json")
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}

	body, err := json.MarshalIndent(checkin.Raw, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal checkin %s: %w", checkin.ID, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return false, fmt.Errorf("write checkin %s: %w", checkin.ID, err)
	}
	return true, nil
}

func (s *Store) writeState(state State) error {
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(s.statePath, body, 0o644)
}

func (s *Store) readState() (*State, error) {
	body, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	return &state, nil
}

func (s *Store) countFiles() int {
	entries, err := os.ReadDir(s.checkinsDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count
}

func (s *Store) countArchiveRows() int {
	if s.db == nil {
		return s.countFiles()
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM checkins`).Scan(&count); err != nil {
		return s.countFiles()
	}
	return count
}
