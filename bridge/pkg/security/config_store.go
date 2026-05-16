package security

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

type CustomPattern struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	Pattern    string `json:"pattern"`
	Compiled   *regexp.Regexp
	IsActive   bool   `json:"is_active"`
}

type ConfigStore struct {
	mu       sync.RWMutex
	db       *sql.DB
	secCfg   *SecurityConfig
	patterns map[string]*CustomPattern
}

type ConfigStoreConfig struct {
	Path string
	SecurityConfig *SecurityConfig
}

func NewConfigStore(cfg ConfigStoreConfig) (*ConfigStore, error) {
	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("security_config: open db: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS category_configs (
			category TEXT PRIMARY KEY,
			config_json TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS custom_patterns (
			id TEXT PRIMARY KEY,
			category TEXT NOT NULL,
			pattern TEXT NOT NULL,
			is_active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("security_config: create tables: %w", err)
	}

	store := &ConfigStore{
		db:       db,
		secCfg:   cfg.SecurityConfig,
		patterns: make(map[string]*CustomPattern),
	}

	if err := store.loadPatterns(); err != nil {
		db.Close()
		return nil, fmt.Errorf("security_config: load patterns: %w", err)
	}

	return store, nil
}

func (s *ConfigStore) Close() error {
	return s.db.Close()
}

func (s *ConfigStore) loadPatterns() error {
	rows, err := s.db.Query(`SELECT id, category, pattern, is_active FROM custom_patterns`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p CustomPattern
		var isActive int
		if err := rows.Scan(&p.ID, &p.Category, &p.Pattern, &isActive); err != nil {
			return err
		}
		p.IsActive = isActive == 1
		re, err := regexp.Compile(p.Pattern)
		if err == nil {
			p.Compiled = re
		}
		s.patterns[p.ID] = &p
	}
	return rows.Err()
}

func (s *ConfigStore) SetCategoryConfig(_ context.Context, category DataCategory, cfg *CategoryConfig, updatedBy string) error {
	cfg.ConfiguredAt = time.Now()
	cfg.ConfiguredBy = updatedBy

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(
		`INSERT INTO category_configs (category, config_json, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(category) DO UPDATE SET config_json = ?, updated_at = ?`,
		string(category), string(cfgJSON), time.Now().Unix(),
		string(cfgJSON), time.Now().Unix(),
	)
	if err != nil {
		return err
	}

	if s.secCfg != nil {
		s.secCfg.mu.Lock()
		if s.secCfg.Categories == nil {
			s.secCfg.Categories = make(map[DataCategory]*CategoryConfig)
		}
		s.secCfg.Categories[category] = cfg
		s.secCfg.mu.Unlock()
	}

	return nil
}

func (s *ConfigStore) GetCategoryConfig(_ context.Context, category DataCategory) (*CategoryConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cfgJSON string
	var updatedAt int64
	err := s.db.QueryRow(`SELECT config_json, updated_at FROM category_configs WHERE category = ?`, string(category)).Scan(&cfgJSON, &updatedAt)
	if err == sql.ErrNoRows {
		if s.secCfg != nil {
			s.secCfg.mu.RLock()
			defer s.secCfg.mu.RUnlock()
			if cfg, ok := s.secCfg.Categories[category]; ok {
				return cfg, nil
			}
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg CategoryConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (s *ConfigStore) AddCustomPattern(_ context.Context, p *CustomPattern) error {
	if p.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}

	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	p.Compiled = re

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(
		`INSERT INTO custom_patterns (id, category, pattern, is_active) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET pattern = ?, is_active = ?`,
		p.ID, p.Category, p.Pattern, boolToInt(p.IsActive),
		p.Pattern, boolToInt(p.IsActive),
	)
	if err != nil {
		return err
	}

	s.patterns[p.ID] = p
	return nil
}

func (s *ConfigStore) ListCustomPatterns(_ context.Context, category string) ([]CustomPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []CustomPattern
	for _, p := range s.patterns {
		if category == "" || p.Category == category {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (s *ConfigStore) DeleteCustomPattern(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM custom_patterns WHERE id = ?`, id)
	if err != nil {
		return err
	}
	delete(s.patterns, id)
	return nil
}

func (s *ConfigStore) MatchCustomPatterns(input, category string) []CustomPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []CustomPattern
	for _, p := range s.patterns {
		if !p.IsActive || p.Compiled == nil {
			continue
		}
		if category != "" && p.Category != category {
			continue
		}
		if p.Compiled.MatchString(input) {
			matches = append(matches, *p)
		}
	}
	return matches
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
