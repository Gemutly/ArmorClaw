package secretary

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

//=============================================================================
// Artifact Store
//=============================================================================

const (
	defaultArtifactTTL     = 7 * 24 * time.Hour // 7 days
	maxArtifactContentSize = 50 * 1024 * 1024    // 50 MB
)

// ArtifactStoreConfig holds configuration for the artifact store.
type ArtifactStoreConfig struct {
	Path string
	TTL  time.Duration
}

// ArtifactStore persists artifact envelopes in SQLite with TTL cleanup.
type ArtifactStore struct {
	db  *sql.DB
	ttl time.Duration
}

// NewArtifactStore creates a new artifact store. If cfg.Path is empty,
// uses an in-memory database.
func NewArtifactStore(cfg ArtifactStoreConfig) (*ArtifactStore, error) {
	if cfg.Path == "" {
		cfg.Path = ":memory:"
	}
	if cfg.TTL == 0 {
		cfg.TTL = defaultArtifactTTL
	}

	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("artifact store: open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("artifact store: enable foreign keys: %w", err)
	}

	s := &ArtifactStore{db: db, ttl: cfg.TTL}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("artifact store: init schema: %w", err)
	}

	return s, nil
}

func (s *ArtifactStore) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS artifact_envelopes (
			id          TEXT PRIMARY KEY,
			version     TEXT NOT NULL DEFAULT '1.0',
			status      TEXT NOT NULL DEFAULT 'pending',
			metadata    TEXT NOT NULL DEFAULT '{}',
			owner       TEXT NOT NULL,
			workflow_id TEXT NOT NULL,
			step_id     TEXT DEFAULT '',
			checksum    TEXT DEFAULT '',
			created_at  INTEGER NOT NULL,
			updated_at  INTEGER NOT NULL,
			expires_at  INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_artifacts_workflow ON artifact_envelopes(workflow_id);
		CREATE INDEX IF NOT EXISTS idx_artifacts_owner ON artifact_envelopes(owner);
		CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifact_envelopes(status);
		CREATE INDEX IF NOT EXISTS idx_artifacts_expires ON artifact_envelopes(expires_at);
	`)
	return err
}

// Close closes the underlying database connection.
func (s *ArtifactStore) Close() error {
	return s.db.Close()
}

// StoreEnvelope persists an artifact envelope.
func (s *ArtifactStore) StoreEnvelope(ctx context.Context, env *ArtifactEnvelope) error {
	metaJSON, err := json.Marshal(env.Metadata)
	if err != nil {
		return fmt.Errorf("artifact store: marshal metadata: %w", err)
	}

	expiresAt := env.CreatedAt.Add(s.ttl).Unix()

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO artifact_envelopes
			(id, version, status, metadata, owner, workflow_id, step_id, checksum, created_at, updated_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		env.ID, env.Version, string(env.Status), string(metaJSON),
		env.Owner, env.WorkflowID, env.StepID, env.Checksum,
		env.CreatedAt.Unix(), env.UpdatedAt.Unix(), expiresAt,
	)
	if err != nil {
		return fmt.Errorf("artifact store: insert: %w", err)
	}
	return nil
}

// GetEnvelope retrieves an artifact envelope by ID.
func (s *ArtifactStore) GetEnvelope(ctx context.Context, id string) (*ArtifactEnvelope, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, version, status, metadata, owner, workflow_id, step_id, checksum, created_at, updated_at
		 FROM artifact_envelopes WHERE id = ?`, id)

	var env ArtifactEnvelope
	var statusStr, metaStr string
	var stepID, checksum sql.NullString
	var createdAtUnix, updatedAtUnix int64

	err := row.Scan(&env.ID, &env.Version, &statusStr, &metaStr, &env.Owner,
		&env.WorkflowID, &stepID, &checksum,
		&createdAtUnix, &updatedAtUnix)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("artifact store: query: %w", err)
	}

	env.Status = ArtifactStatus(statusStr)
	env.StepID = stepID.String
	env.Checksum = checksum.String
	env.CreatedAt = time.Unix(createdAtUnix, 0)
	env.UpdatedAt = time.Unix(updatedAtUnix, 0)

	if err := json.Unmarshal([]byte(metaStr), &env.Metadata); err != nil {
		return nil, fmt.Errorf("artifact store: unmarshal metadata: %w", err)
	}

	return &env, nil
}

// ArtifactFilter filters artifact listing queries.
type ArtifactFilter struct {
	Status     *ArtifactStatus
	Owner      string
	WorkflowID string
}

// ListEnvelopes queries artifact envelopes matching the filter.
func (s *ArtifactStore) ListEnvelopes(ctx context.Context, filter ArtifactFilter) ([]ArtifactEnvelope, error) {
	query := `SELECT id, version, status, metadata, owner, workflow_id, step_id, checksum, created_at, updated_at
			  FROM artifact_envelopes WHERE 1=1`
	args := []interface{}{}

	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*filter.Status))
	}
	if filter.Owner != "" {
		query += " AND owner = ?"
		args = append(args, filter.Owner)
	}
	if filter.WorkflowID != "" {
		query += " AND workflow_id = ?"
		args = append(args, filter.WorkflowID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("artifact store: list query: %w", err)
	}
	defer rows.Close()

	var results []ArtifactEnvelope
	for rows.Next() {
		var env ArtifactEnvelope
		var statusStr, metaStr string
		var stepID, checksum sql.NullString
		var createdAtUnix, updatedAtUnix int64

		if err := rows.Scan(&env.ID, &env.Version, &statusStr, &metaStr, &env.Owner,
			&env.WorkflowID, &stepID, &checksum,
			&createdAtUnix, &updatedAtUnix); err != nil {
			return nil, fmt.Errorf("artifact store: scan row: %w", err)
		}

		env.Status = ArtifactStatus(statusStr)
		env.StepID = stepID.String
		env.Checksum = checksum.String
		env.CreatedAt = time.Unix(createdAtUnix, 0)
		env.UpdatedAt = time.Unix(updatedAtUnix, 0)

		if err := json.Unmarshal([]byte(metaStr), &env.Metadata); err != nil {
			log.Printf("artifact store: skipping bad metadata for %s: %v", env.ID, err)
			continue
		}

		results = append(results, env)
	}

	return results, rows.Err()
}

// UpdateStatus transitions an artifact's status.
func (s *ArtifactStore) UpdateStatus(ctx context.Context, id string, newStatus ArtifactStatus) error {
	now := time.Now().UTC().Unix()
	result, err := s.db.ExecContext(ctx,
		`UPDATE artifact_envelopes SET status = ?, updated_at = ? WHERE id = ?`,
		string(newStatus), now, id)
	if err != nil {
		return fmt.Errorf("artifact store: update status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("artifact store: envelope %q not found", id)
	}
	return nil
}

// CleanupExpired removes all expired artifacts. Returns count of removed items.
func (s *ArtifactStore) CleanupExpired(ctx context.Context) (int, error) {
	now := time.Now().UTC().Unix()
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM artifact_envelopes WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("artifact store: cleanup: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}
