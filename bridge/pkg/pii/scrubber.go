// Package pii provides PII detection and scrubbing for ArmorClaw
package pii

import (
	"regexp"
	"strings"
	"sync"
)

// PIIPattern defines a PII detection pattern
type PIIPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Description string
}

// Scrubber detects and redacts PII from text
type Scrubber struct {
	patterns    []*PIIPattern
	patternsMap map[string]*PIIPattern
	mu          sync.RWMutex
	enabled     bool

	// Sealed keystore integration (nil = no gate)
	sealedCheck func() error
	resetTimer  func()
}

// Redaction holds information about a redaction
type Redaction struct {
	Type        string
	Start       int
	End         int
	Original    string
	Replacement string
	Description string
}

// New creates a new PII scrubber with default patterns
func New() *Scrubber {
	return NewWithPatterns(DefaultPatterns())
}

// NewWithPatterns creates a scrubber with custom patterns
func NewWithPatterns(patterns []*PIIPattern) *Scrubber {
	patternsMap := make(map[string]*PIIPattern)
	for _, p := range patterns {
		patternsMap[p.Name] = p
	}

	return &Scrubber{
		patterns:    patterns,
		patternsMap: patternsMap,
		enabled:     true,
	}
}

// DefaultPatterns returns the default PII detection patterns
// Order matters: more specific patterns must come before generic ones
func DefaultPatterns() []*PIIPattern {
	return []*PIIPattern{
		// Credit cards FIRST (before phone pattern)
		{
			Name:        "credit_card",
			Pattern:     regexp.MustCompile(`\b4[0-9]{12}(?:[0-9]{3})?\b|\b5[1-5][0-9]{14}\b|\b6(?:011|5[0-9][0-9])[0-9]{12}\b|\b3[47][0-9]{13}\b`),
			Replacement: "[REDACTED_CREDIT_CARD]",
			Description: "Credit card numbers (Visa, MC, Amex, Discover)",
		},
		// Phone (10-digit format - after credit cards)
		{
			Name:        "phone",
			Pattern:     regexp.MustCompile(`(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`),
			Replacement: "[REDACTED_PHONE]",
			Description: "US phone numbers (10-digit format with word boundary)",
		},
		// SSN
		{
			Name:        "ssn",
			Pattern:     regexp.MustCompile(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`),
			Replacement: "[REDACTED_SSN]",
			Description: "US Social Security Numbers (XXX-XX-XXXX format)",
		},
		// JWT tokens (very specific format)
		{
			Name:        "bearer_token",
			Pattern:     regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
			Replacement: "[REDACTED_JWT]",
			Description: "JWT bearer tokens (3-part format)",
		},
		// API keys with specific prefixes (before generic token pattern)
		{
			Name:        "github_token",
			Pattern:     regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36,}\b`),
			Replacement: "[REDACTED_GITHUB_TOKEN]",
			Description: "GitHub personal access tokens (36-40 chars)",
		},
		{
			Name:        "aws_key_id",
			Pattern:     regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
			Replacement: "[REDACTED_AWS_KEY]",
			Description: "AWS access key IDs",
		},
		{
			Name:        "api_key_sk",
			Pattern:     regexp.MustCompile(`\bsk_[a-zA-Z0-9_-]{20,}\b`),
			Replacement: "[REDACTED_API_KEY]",
			Description: "Stripe API keys (sk_ prefix)",
		},
		{
			Name:        "api_key_pk",
			Pattern:     regexp.MustCompile(`\bpk_[a-zA-Z0-9_-]{20,}\b`),
			Replacement: "[REDACTED_API_KEY]",
			Description: "Stripe publishable keys (pk_ prefix)",
		},
		{
			Name:        "api_key_ai",
			Pattern:     regexp.MustCompile(`\bai_[a-zA-Z0-9_-]{10,}\b`),
			Replacement: "[REDACTED_API_KEY]",
			Description: "OpenAI API keys (ai_ prefix with hyphens)",
		},
		// Generic token before AWS secret (more specific pattern)
		{
			Name:        "api_key_generic",
			Pattern:     regexp.MustCompile(`\b[a-zA-Z0-9_-]{40,}\b`),
			Replacement: "[REDACTED_TOKEN]",
			Description: "Generic API tokens (40+ chars)",
		},
		// AWS secret (specific format - base64)
		{
			Name:        "aws_secret",
			Pattern:     regexp.MustCompile(`\b[0-9a-zA-Z/+]{40}\b`),
			Replacement: "[REDACTED_AWS_SECRET]",
			Description: "AWS secret access keys (40 base64 chars)",
		},
		// Email
		{
			Name:        "email",
			Pattern:     regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`),
			Replacement: "[REDACTED_EMAIL]",
			Description: "Email addresses",
		},
		// IP addresses (after email to avoid matching local parts)
		{
			Name:        "ip_address",
			Pattern:     regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`),
			Replacement: "[REDACTED_IP]",
			Description: "IPv4 addresses",
		},
		// Config-style secrets
		{
			Name:        "password",
			Pattern:     regexp.MustCompile(`(?i)password\s*[=:]\s*[^\s]+`),
			Replacement: "password=[REDACTED]",
			Description: "Password in configuration",
		},
		{
			Name:        "token",
			Pattern:     regexp.MustCompile(`(?i)token\s*[=:]\s*[^\s]+`),
			Replacement: "token=[REDACTED]",
			Description: "Token in configuration",
		},
		{
			Name:        "secret",
			Pattern:     regexp.MustCompile(`(?i)secret\s*[=:]\s*[^\s]+`),
			Replacement: "secret=[REDACTED]",
			Description: "Secret in configuration",
		},
		// Generic token pattern LAST (most permissive, matches many things)
		{
			Name:        "api_key_generic",
			Pattern:     regexp.MustCompile(`\b[a-zA-Z0-9_-]{40,}\b`),
			Replacement: "[REDACTED_TOKEN]",
			Description: "Generic API tokens (40+ chars)",
		},
	}
}

// Scrub removes PII from text, returning redacted version and list of redactions
func (s *Scrubber) Scrub(text string) (string, []Redaction) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return text, nil
	}

	result := text
	redactions := []Redaction{}
	replacedRanges := make(map[[2]int]bool)

	// Find all matches for each pattern
	for _, pii := range s.patterns {
		matches := pii.Pattern.FindAllStringIndex(result, -1)
		for _, match := range matches {
			start, end := match[0], match[1]

			// Skip if this range overlaps with already replaced text
			overlaps := false
			for rng := range replacedRanges {
				if start < rng[1] && end > rng[0] {
					overlaps = true
					break
				}
			}
			if overlaps {
				continue
			}

			if start >= len(result) || end > len(result) || start < 0 {
				continue
			}

			original := result[start:end]
			replacement := pii.Replacement

			redactions = append(redactions, Redaction{
				Type:        pii.Name,
				Start:       start,
				End:         end,
				Original:    original,
				Replacement: replacement,
				Description: pii.Description,
			})

			// Mark this range as replaced
			replacedRanges[[2]int{start, end}] = true
		}
	}

	// Sort redactions by start position (reverse order) for replacement
	for i := 0; i < len(redactions); i++ {
		for j := i + 1; j < len(redactions); j++ {
			if redactions[i].Start < redactions[j].Start {
				redactions[i], redactions[j] = redactions[j], redactions[i]
			}
		}
	}

	// Apply replacements from end to start to prevent index shifting
	for _, redaction := range redactions {
		if redaction.Start >= 0 && redaction.End <= len(result) {
			result = result[:redaction.Start] + redaction.Replacement + result[redaction.End:]
		}
	}

	return result, redactions
}

// ScrubMap scrubs PII from map[string]interface{} values (for JSON payloads)
func (s *Scrubber) ScrubMap(data map[string]interface{}) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return data
	}

	result := make(map[string]interface{})
	for key, value := range data {
		switch v := value.(type) {
		case string:
			scrubbed, _ := s.Scrub(v)
			result[key] = scrubbed
		case map[string]interface{}:
			result[key] = s.ScrubMap(v)
		case []interface{}:
			result[key] = s.ScrubSlice(v)
		default:
			result[key] = value
		}
	}
	return result
}

// ScrubSlice scrubs PII from slices
func (s *Scrubber) ScrubSlice(slice []interface{}) []interface{} {
	result := make([]interface{}, len(slice))
	for i, value := range slice {
		switch v := value.(type) {
		case string:
			scrubbed, _ := s.Scrub(v)
			result[i] = scrubbed
		case map[string]interface{}:
			result[i] = s.ScrubMap(v)
		case []interface{}:
			result[i] = s.ScrubSlice(v)
		default:
			result[i] = value
		}
	}
	return result
}

// Detect scans text for PII and returns the detected patterns
func (s *Scrubber) Detect(text string) []Redaction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return nil
	}

	detections := []Redaction{}

	// Find all matches for each pattern
	for _, pii := range s.patterns {
		matches := pii.Pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			if start >= len(text) || end > len(text) {
				continue
			}

			original := text[start:end]
			detections = append(detections, Redaction{
				Type:        pii.Name,
				Start:       start,
				End:         end,
				Original:    original,
				Replacement: pii.Replacement,
				Description: pii.Description,
			})
		}
	}

	return detections
}

// ContainsPII checks if text contains any PII patterns
func (s *Scrubber) ContainsPII(text string) bool {
	return len(s.Detect(text)) > 0
}

// AddPattern adds a custom PII pattern
func (s *Scrubber) AddPattern(name, pattern, replacement, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	pii := &PIIPattern{
		Name:        name,
		Pattern:     re,
		Replacement: replacement,
		Description: description,
	}

	s.patterns = append(s.patterns, pii)
	s.patternsMap[name] = pii

	return nil
}

// RemovePattern removes a PII pattern by name
func (s *Scrubber) RemovePattern(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.patterns {
		if p.Name == name {
			s.patterns = append(s.patterns[:i], s.patterns[i+1:]...)
			break
		}
	}
	delete(s.patternsMap, name)
}

// SetEnabled enables or disables the scrubber
func (s *Scrubber) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// IsEnabled returns whether the scrubber is enabled
func (s *Scrubber) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GetPatternCount returns the number of active patterns
func (s *Scrubber) GetPatternCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.patterns)
}

// GetPatternNames returns a list of active pattern names
func (s *Scrubber) GetPatternNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, len(s.patterns))
	for i, p := range s.patterns {
		names[i] = p.Name
	}
	return names
}

// SanitizeForLog scrubs text specifically for logging (more aggressive)
func (s *Scrubber) SanitizeForLog(text string) string {
	if !s.enabled {
		return text
	}

	// Use a more aggressive pattern set for logging
	result := text

	// Redact common password patterns
	result = regexp.MustCompile(`(?i)password["']?\s*[:=]\s*["']?[^\s"']+`).ReplaceAllString(result, "password=[REDACTED]")
	result = regexp.MustCompile(`(?i)passwd["']?\s*[:=]\s*["']?[^\s"']+`).ReplaceAllString(result, "passwd=[REDACTED]")

	// Redact tokens
	result = regexp.MustCompile(`(?i)token["']?\s*[:=]\s*["']?[^\s"']+`).ReplaceAllString(result, "token=[REDACTED]")

	// Redact secrets
	result = regexp.MustCompile(`(?i)secret["']?\s*[:=]\s*["']?[^\s"']+`).ReplaceAllString(result, "secret=[REDACTED]")

	// Redact API keys with common prefixes
	result = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token)["']?\s*[:=]\s*["']?[^\s"']+`).ReplaceAllString(result, "$1=[REDACTED]")

	// Redact connection strings
	result = regexp.MustCompile(`(?i)(mongodb|mysql|postgres|redis)://[^\s]+`).ReplaceAllString(result, "$1://[REDACTED]")

	// Redact bearer tokens (JWT format) - replace entire "Bearer TOKEN" with "[REDACTED_JWT]"
	result = regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`).ReplaceAllString(result, "[REDACTED_JWT]")

	// Apply standard PII scrubbing
	result, _ = s.Scrub(result)

	return result
}

// ScrubBytes scrubs PII from byte slices
func (s *Scrubber) ScrubBytes(data []byte) []byte {
	text := string(data)
	scrubbed, _ := s.Scrub(text)
	return []byte(scrubbed)
}

// CreateSummary creates a summary of redactions for logging
func (s *Scrubber) CreateSummary(redactions []Redaction) map[string]int {
	summary := make(map[string]int)
	for _, r := range redactions {
		summary[r.Type]++
	}
	return summary
}

// ValidatePattern checks if a pattern string is valid
func ValidatePattern(pattern string) error {
	_, err := regexp.Compile(pattern)
	return err
}

// GetPattern returns a pattern by name
func (s *Scrubber) GetPattern(name string) (*PIIPattern, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.patternsMap[name]
	return p, ok
}

// HasPattern checks if a pattern exists
func (s *Scrubber) HasPattern(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.patternsMap[name]
	return ok
}

// Clone creates a copy of the scrubber
func (s *Scrubber) Clone() *Scrubber {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deep copy patterns slice and create new map entries
	patterns := make([]*PIIPattern, len(s.patterns))
	patternsMap := make(map[string]*PIIPattern, len(s.patternsMap))

	for i, p := range s.patterns {
		// Copy the pattern (PIIPattern contains immutable fields except Pattern which is a regexp.Regexp)
		// regexp.Regexp is safe to share between goroutines, so we can reuse the pointer
		patterns[i] = p
		patternsMap[p.Name] = p
	}

	return &Scrubber{
		patterns:    patterns,
		patternsMap: patternsMap,
		enabled:     s.enabled,
	}
}

// Reset resets the scrubber to default patterns
func (s *Scrubber) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.patterns = DefaultPatterns()
	s.patternsMap = make(map[string]*PIIPattern)
	for _, p := range s.patterns {
		s.patternsMap[p.Name] = p
	}
	s.enabled = true
}

// MaskString masks a string with asterisks (for partial redaction)
func MaskString(s string, visibleChars int) string {
	if len(s) <= visibleChars {
		return strings.Repeat("*", len(s))
	}
	// Calculate asterisks correctly: total length minus visible characters
	asteriskCount := len(s) - visibleChars
	return s[:visibleChars] + strings.Repeat("*", asteriskCount)
}

// MaskEmail masks an email address (shows first 2 chars of user, or all * if shorter)
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	user := parts[0]
	domain := parts[1]

	// Mask user: show first 2 chars or all * if 2 or fewer (minimum 2 asterisks)
	if len(user) <= 2 {
		// Use at least 2 asterisks to avoid revealing exact length
		maskLen := len(user)
		if maskLen < 2 {
			maskLen = 2
		}
		user = strings.Repeat("*", maskLen)
	} else {
		user = user[:2] + strings.Repeat("*", len(user)-2)
	}

	// Mask domain TLD
	domainParts := strings.Split(domain, ".")
	if len(domainParts) > 1 {
		domainParts[len(domainParts)-1] = "***"
		domain = strings.Join(domainParts, ".")
	}

	return user + "@" + domain
}

// MaskPhone masks a phone number (shows last 4 digits)
func MaskPhone(phone string) string {
	// Remove all non-digits
	digits := regexp.MustCompile(`[^\d]`).ReplaceAllString(phone, "")
	if len(digits) < 4 {
		return "***-***-****"
	}
	visible := digits[len(digits)-4:]
	return "***-***-" + visible
}

// SetSealedCheck configures the sealed keystore gate for PII fulfillment.
// When set, Fulfill will return the sealed error instead of returning data.
func (s *Scrubber) SetSealedCheck(check func() error, reset func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealedCheck = check
	s.resetTimer = reset
}

// Fulfill scrubs PII from text after verifying the keystore is unsealed.
// This is an activity operation — it resets the auto-seal timer on success.
// Returns ErrPasswordSealed (-32005) if the keystore is sealed.
func (s *Scrubber) Fulfill(text string) (string, []Redaction, error) {
	s.mu.RLock()
	check := s.sealedCheck
	reset := s.resetTimer
	s.mu.RUnlock()

	if check != nil {
		if err := check(); err != nil {
			return "", nil, err
		}
	}

	result, redactions := s.Scrub(text)

	if reset != nil {
		reset()
	}

	return result, redactions, nil
}
