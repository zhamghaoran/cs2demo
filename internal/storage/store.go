package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/cs2demo/platform/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db      *sql.DB
	dataDir string
}

func Open(sqlitePath, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite3", sqlitePath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &Store{db: db, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS demos (
  id          TEXT PRIMARY KEY,
  filename    TEXT NOT NULL,
  file_path   TEXT NOT NULL,
  target_user TEXT NOT NULL,
  status      TEXT NOT NULL,
  error       TEXT,
  candidates  TEXT,
  stats_json  TEXT,
  report_json TEXT,
  created_at  DATETIME NOT NULL,
  updated_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_demos_status ON demos(status);
`)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`ALTER TABLE demos ADD COLUMN candidates TEXT`)
	return nil
}

func (s *Store) SaveUpload(id, filename string, src io.Reader) (string, error) {
	dir := filepath.Join(s.dataDir, "demos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".dem")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, src); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) CreateDemo(ctx context.Context, d domain.Demo) error {
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
INSERT INTO demos(id, filename, file_path, target_user, status, error, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?)`,
		d.ID, d.Filename, d.FilePath, d.TargetUser, d.Status, d.Error, d.CreatedAt, d.UpdatedAt)
	return err
}

func (s *Store) UpdateStatus(ctx context.Context, id string, status domain.DemoStatus, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE demos SET status=?, error=?, updated_at=? WHERE id=?`,
		status, errMsg, time.Now().UTC(), id)
	return err
}

func (s *Store) UpdateFailureWithCandidates(ctx context.Context, id string, errMsg string, candidates []string) error {
	b, _ := json.Marshal(candidates)
	_, err := s.db.ExecContext(ctx, `
UPDATE demos SET status=?, error=?, candidates=?, updated_at=? WHERE id=?`,
		domain.StatusFailed, errMsg, string(b), time.Now().UTC(), id)
	return err
}

func (s *Store) SaveStats(ctx context.Context, id string, stats domain.MatchStats) error {
	b, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE demos SET stats_json=?, updated_at=? WHERE id=?`,
		string(b), time.Now().UTC(), id)
	return err
}

func (s *Store) SaveReport(ctx context.Context, id string, r domain.AnalysisReport) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE demos SET report_json=?, updated_at=? WHERE id=?`,
		string(b), time.Now().UTC(), id)
	return err
}

func (s *Store) GetDemo(ctx context.Context, id string) (domain.Demo, error) {
	var d domain.Demo
	var errMsg, candRaw sql.NullString
	row := s.db.QueryRowContext(ctx, `
SELECT id, filename, file_path, target_user, status, COALESCE(error,''), COALESCE(candidates,''), created_at, updated_at
FROM demos WHERE id=?`, id)
	err := row.Scan(&d.ID, &d.Filename, &d.FilePath, &d.TargetUser, &d.Status, &errMsg, &candRaw, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.Error = errMsg.String
	if candRaw.Valid && candRaw.String != "" {
		_ = json.Unmarshal([]byte(candRaw.String), &d.Candidates)
	}
	return d, nil
}

func (s *Store) GetReport(ctx context.Context, id string) (domain.AnalysisReport, bool, error) {
	var raw sql.NullString
	row := s.db.QueryRowContext(ctx, `SELECT report_json FROM demos WHERE id=?`, id)
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.AnalysisReport{}, false, ErrNotFound
		}
		return domain.AnalysisReport{}, false, err
	}
	if !raw.Valid || raw.String == "" {
		return domain.AnalysisReport{}, false, nil
	}
	var r domain.AnalysisReport
	if err := json.Unmarshal([]byte(raw.String), &r); err != nil {
		return domain.AnalysisReport{}, false, err
	}
	return r, true, nil
}

func (s *Store) GetStats(ctx context.Context, id string) (domain.MatchStats, bool, error) {
	var raw sql.NullString
	row := s.db.QueryRowContext(ctx, `SELECT stats_json FROM demos WHERE id=?`, id)
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.MatchStats{}, false, ErrNotFound
		}
		return domain.MatchStats{}, false, err
	}
	if !raw.Valid || raw.String == "" {
		return domain.MatchStats{}, false, nil
	}
	var st domain.MatchStats
	if err := json.Unmarshal([]byte(raw.String), &st); err != nil {
		return domain.MatchStats{}, false, err
	}
	return st, true, nil
}

func (s *Store) ListDemos(ctx context.Context, limit int) ([]domain.Demo, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, filename, file_path, target_user, status, COALESCE(error,''), COALESCE(candidates,''), created_at, updated_at
FROM demos ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Demo
	for rows.Next() {
		var d domain.Demo
		var errMsg, candRaw sql.NullString
		if err := rows.Scan(&d.ID, &d.Filename, &d.FilePath, &d.TargetUser, &d.Status, &errMsg, &candRaw, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Error = errMsg.String
		if candRaw.Valid && candRaw.String != "" {
			_ = json.Unmarshal([]byte(candRaw.String), &d.Candidates)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
