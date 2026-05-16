package email

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

const (
	StatusQueued            = "queued"
	StatusAwaitingApproval  = "awaiting_approval"
	StatusApproved          = "approved"
	StatusSending           = "sending"
	StatusSent              = "sent"
	StatusRetryWait         = "retry_wait"
	StatusFailed            = "failed"
	StatusDeadLetter        = "dead_letter"

	MaxRetryAttempts = 5
)

var validStatusTransitions = map[string][]string{
	StatusQueued:           {StatusAwaitingApproval, StatusSending},
	StatusAwaitingApproval: {StatusApproved, StatusFailed},
	StatusApproved:         {StatusSending},
	StatusSending:          {StatusSent, StatusRetryWait, StatusFailed},
	StatusRetryWait:        {StatusSending, StatusDeadLetter},
	StatusFailed:           {StatusDeadLetter},
	StatusSent:             {},
	StatusDeadLetter:       {},
}

type OutboxEntry struct {
	ID             string `json:"id"`
	WorkflowID     string `json:"workflow_id"`
	MessageID      string `json:"message_id,omitempty"`
	Status         string `json:"status"`
	AttemptCount   int    `json:"attempt_count"`
	NextAttemptAt  *int64 `json:"next_attempt_at,omitempty"`
	LastErrorCode  string `json:"last_error_code,omitempty"`
	RecipientHash  string `json:"recipient_hash"`
	SubjectHash    string `json:"subject_hash,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type OutboxStore struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewOutboxStore(dbPath string) (*OutboxStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("outbox: open db: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS email_outbox (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			message_id TEXT,
			status TEXT NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			next_attempt_at INTEGER,
			last_error_code TEXT,
			recipient_hash TEXT NOT NULL,
			subject_hash TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_outbox_status ON email_outbox(status);
		CREATE INDEX IF NOT EXISTS idx_outbox_workflow ON email_outbox(workflow_id);
		CREATE INDEX IF NOT EXISTS idx_outbox_next_attempt ON email_outbox(next_attempt_at) WHERE status = 'retry_wait'
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("outbox: create schema: %w", err)
	}

	return &OutboxStore{db: db}, nil
}

func (s *OutboxStore) Close() error {
	return s.db.Close()
}

func (s *OutboxStore) Enqueue(ctx context.Context, entry *OutboxEntry) error {
	if entry.Status == "" {
		entry.Status = StatusQueued
	}
	now := time.Now().Unix()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO email_outbox (id, workflow_id, message_id, status, attempt_count, next_attempt_at, last_error_code, recipient_hash, subject_hash, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.WorkflowID, entry.MessageID, entry.Status, entry.AttemptCount,
		entry.NextAttemptAt, entry.LastErrorCode, entry.RecipientHash, entry.SubjectHash,
		entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("outbox enqueue: %w", err)
	}
	return nil
}

func (s *OutboxStore) GetByID(ctx context.Context, id string) (*OutboxEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workflow_id, message_id, status, attempt_count, next_attempt_at, last_error_code, recipient_hash, subject_hash, created_at, updated_at
		 FROM email_outbox WHERE id = ?`, id)

	var entry OutboxEntry
	err := row.Scan(&entry.ID, &entry.WorkflowID, &entry.MessageID, &entry.Status,
		&entry.AttemptCount, &entry.NextAttemptAt, &entry.LastErrorCode,
		&entry.RecipientHash, &entry.SubjectHash, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("outbox get: %w", err)
	}
	return &entry, nil
}

func (s *OutboxStore) UpdateStatus(ctx context.Context, id, newStatus string) error {
	entry, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("outbox entry %q not found", id)
	}

	if !isValidTransition(entry.Status, newStatus) {
		return fmt.Errorf("invalid status transition: %s -> %s", entry.Status, newStatus)
	}

	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx,
		`UPDATE email_outbox SET status = ?, updated_at = ? WHERE id = ?`,
		newStatus, now, id)
	if err != nil {
		return fmt.Errorf("outbox update status: %w", err)
	}
	return nil
}

func (s *OutboxStore) IncrementAttempt(ctx context.Context, id string, errorCode string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE email_outbox SET attempt_count = attempt_count + 1, last_error_code = ?, updated_at = ? WHERE id = ?`,
		errorCode, now, id)
	if err != nil {
		return fmt.Errorf("outbox increment attempt: %w", err)
	}
	return nil
}

func (s *OutboxStore) MarkDeadLetter(ctx context.Context, id string) error {
	return s.UpdateStatus(ctx, id, StatusDeadLetter)
}

func (s *OutboxStore) ListByStatus(ctx context.Context, status string, limit, offset int) ([]*OutboxEntry, error) {
	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, workflow_id, message_id, status, attempt_count, next_attempt_at, last_error_code, recipient_hash, subject_hash, created_at, updated_at
			 FROM email_outbox WHERE status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			status, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, workflow_id, message_id, status, attempt_count, next_attempt_at, last_error_code, recipient_hash, subject_hash, created_at, updated_at
			 FROM email_outbox ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("outbox list: %w", err)
	}
	defer rows.Close()

	var entries []*OutboxEntry
	for rows.Next() {
		var entry OutboxEntry
		if err := rows.Scan(&entry.ID, &entry.WorkflowID, &entry.MessageID, &entry.Status,
			&entry.AttemptCount, &entry.NextAttemptAt, &entry.LastErrorCode,
			&entry.RecipientHash, &entry.SubjectHash, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("outbox scan: %w", err)
		}
		entries = append(entries, &entry)
	}
	return entries, rows.Err()
}

func (s *OutboxStore) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM email_outbox GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("outbox stats: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	total := 0
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("outbox stats scan: %w", err)
		}
		counts[status] = count
		total += count
	}

	return map[string]interface{}{
		"total":     total,
		"by_status": counts,
	}, rows.Err()
}

func (s *OutboxStore) Retry(ctx context.Context, id string) error {
	entry, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("outbox entry %q not found", id)
	}

	if entry.Status != StatusFailed && entry.Status != StatusDeadLetter {
		return fmt.Errorf("can only retry failed or dead_letter emails, got %s", entry.Status)
	}

	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx,
		`UPDATE email_outbox SET status = ?, attempt_count = 0, next_attempt_at = NULL, last_error_code = '', updated_at = ? WHERE id = ?`,
		StatusQueued, now, id)
	if err != nil {
		return fmt.Errorf("outbox retry: %w", err)
	}
	return nil
}

func isValidTransition(from, to string) bool {
	allowed, ok := validStatusTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
