package email

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

type RoutingRule struct {
	ID        string `json:"id"`
	Pattern   string `json:"pattern"`
	TeamID    string `json:"team_id"`
	Priority  int    `json:"priority"`
	IsActive  bool   `json:"is_active"`
	MatchField string `json:"match_field"`
}

type RoutingRuleStore struct {
	mu   sync.RWMutex
	db   *sql.DB
	rules map[string]*RoutingRule
}

type RoutingRuleStoreConfig struct {
	Path string
}

func NewRoutingRuleStore(cfg RoutingRuleStoreConfig) (*RoutingRuleStore, error) {
	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("routing_rules: open db: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS routing_rules (
			id TEXT PRIMARY KEY,
			pattern TEXT NOT NULL,
			team_id TEXT NOT NULL,
			priority INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			match_field TEXT DEFAULT 'to'
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("routing_rules: create table: %w", err)
	}

	store := &RoutingRuleStore{
		db:    db,
		rules: make(map[string]*RoutingRule),
	}

	if err := store.loadAll(); err != nil {
		db.Close()
		return nil, fmt.Errorf("routing_rules: load: %w", err)
	}

	return store, nil
}

func (s *RoutingRuleStore) loadAll() error {
	rows, err := s.db.Query(`SELECT id, pattern, team_id, priority, is_active, match_field FROM routing_rules ORDER BY priority DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var r RoutingRule
		var isActive int
		if err := rows.Scan(&r.ID, &r.Pattern, &r.TeamID, &r.Priority, &isActive, &r.MatchField); err != nil {
			return err
		}
		r.IsActive = isActive == 1
		s.rules[r.ID] = &r
	}
	return rows.Err()
}

func (s *RoutingRuleStore) Close() error {
	return s.db.Close()
}

func (s *RoutingRuleStore) CreateRule(_ context.Context, rule *RoutingRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO routing_rules (id, pattern, team_id, priority, is_active, match_field) VALUES (?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.Pattern, rule.TeamID, rule.Priority, boolToInt(rule.IsActive), rule.MatchField,
	)
	if err != nil {
		return err
	}

	s.rules[rule.ID] = rule
	return nil
}

func (s *RoutingRuleStore) GetRule(_ context.Context, id string) (*RoutingRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (s *RoutingRuleStore) ListRules(_ context.Context) ([]RoutingRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RoutingRule, 0, len(s.rules))
	for _, r := range s.rules {
		result = append(result, *r)
	}
	return result, nil
}

func (s *RoutingRuleStore) DeleteRule(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM routing_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	delete(s.rules, id)
	return nil
}

func (s *RoutingRuleStore) Match(ctx context.Context, address, subject string) (*RoutingRule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.rules {
		if !r.IsActive {
			continue
		}
		var input string
		switch r.MatchField {
		case "subject":
			input = subject
		default:
			input = address
		}

		if strings.Contains(r.Pattern, "*") || strings.Contains(r.Pattern, "?") || strings.Contains(r.Pattern, "[") {
			pattern := "^" + wildcardToRegex(r.Pattern) + "$"
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(input) {
				return r, true
			}
		} else if strings.Contains(r.Pattern, "@") && r.MatchField != "subject" {
			if strings.EqualFold(input, r.Pattern) || strings.HasSuffix(strings.ToLower(input), "@"+strings.ToLower(strings.TrimPrefix(r.Pattern, "@"))) {
				return r, true
			}
		} else {
			if strings.EqualFold(input, r.Pattern) {
				return r, true
			}
		}
	}
	return nil, false
}

func wildcardToRegex(pattern string) string {
	var result strings.Builder
	for _, ch := range pattern {
		switch ch {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '(', ')', '^', '$', '+', '{', '}', '|', '\\':
			result.WriteByte('\\')
			result.WriteRune(ch)
		default:
			result.WriteRune(ch)
		}
	}
	return result.String()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
