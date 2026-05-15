package skills

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Skill represents a single skill from a SKILL.md file
type Skill struct {
	Name          string                                    `json:"name"`
	Description   string                                    `json:"description"`
	Homepage      string                                    `json:"homepage"`
	Domain        string                                    `json:"domain"`
	Risk          string                                    `json:"risk"`
	Command       string                                    `json:"command"`
	Enabled       bool                                      `json:"enabled"`
	Timeout       time.Duration                             `json:"timeout"`
	Version       string                                    `json:"version"`
	Parameters    map[string]Param                          `json:"parameters"`
	Metadata      map[string]interface{}                    `json:"metadata"`
	SecurityCheck func(params map[string]interface{}) error `json:"-"`
}

// Param represents a parameter for a skill
type Param struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Validation  string `json:"validation,omitempty"`
}

// Registry manages all available skills
type Registry struct {
	skills      map[string]*Skill
	domainIndex map[string][]*Skill
	mutex       sync.RWMutex
}

// NewRegistry creates a new skill registry
func NewRegistry() *Registry {
	return &Registry{
		skills:      make(map[string]*Skill),
		domainIndex: make(map[string][]*Skill),
	}
}

// ScanSkills scans the skills directory for SKILL.md files
func (r *Registry) ScanSkills(skillsDir string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Clear existing data
	r.skills = make(map[string]*Skill)
	r.domainIndex = make(map[string][]*Skill)

	return filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Name() == "SKILL.md" {
			skill, err := parseSkillFile(path)
			if err != nil {
				log.Printf("Warning: failed to parse skill file %s: %v\n", path, err)
				return nil // Continue scanning other files
			}

			r.skills[skill.Name] = skill

			// Add to domain index
			r.domainIndex[skill.Domain] = append(r.domainIndex[skill.Domain], skill)
		}

		return nil
	})
}

// parseSkillFile parses a single SKILL.md file
func parseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	skill := &Skill{
		Enabled: true, // Skills are enabled by default
	}

	// Parse frontmatter
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("invalid SKILL.md format: no frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SKILL.md format: missing closing ---")
	}

	frontmatter := strings.TrimSpace(parts[1])

	// Parse YAML frontmatter (simplified for Phase 1)
	// In production, use a proper YAML parser like gopkg.in/yaml.v2
	var frontmatterMap map[string]interface{}
	if err := parseYAMLFrontmatter(frontmatter, &frontmatterMap); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	// Extract basic fields
	if name, ok := frontmatterMap["name"].(string); ok {
		skill.Name = name
	} else {
		return nil, fmt.Errorf("skill missing name")
	}

	if desc, ok := frontmatterMap["description"].(string); ok {
		skill.Description = desc
	}

	if homepage, ok := frontmatterMap["homepage"].(string); ok {
		skill.Homepage = homepage
	}

	// Extract domain from skill name or metadata
	skill.Domain = extractDomainFromName(skill.Name)

	// Set risk level based on domain
	skill.Risk = getRiskForDomain(skill.Domain)

	// Extract command
	skill.Command = getCommandForSkill(skill.Name)

	if timeoutVal, ok := frontmatterMap["timeout"]; ok {
		switch v := timeoutVal.(type) {
		case int:
			skill.Timeout = time.Duration(v) * time.Second
		case float64:
			skill.Timeout = time.Duration(v) * time.Second
		case string:
			if dur, err := time.ParseDuration(v); err == nil {
				skill.Timeout = dur
			}
		}
	}

	if version, ok := frontmatterMap["version"].(string); ok {
		skill.Version = version
	}

	// Enabled defaults to true; only override when explicitly set in frontmatter
	if enabled, ok := frontmatterMap["enabled"]; ok {
		switch v := enabled.(type) {
		case bool:
			skill.Enabled = v
		case string:
			skill.Enabled = strings.ToLower(v) == "true"
		}
	}

	skill.Parameters = make(map[string]Param)
	if paramsRaw, ok := frontmatterMap["parameters"]; ok {
		if paramsList, ok := paramsRaw.([]interface{}); ok {
			for _, p := range paramsList {
				if paramMap, ok := p.(map[string]interface{}); ok {
					name, _ := paramMap["name"].(string)
					if name != "" {
						param := Param{
							Type:        getStringFromMap(paramMap, "type"),
							Description: getStringFromMap(paramMap, "description"),
							Required:    getBoolFromMap(paramMap, "required"),
							Validation:  getStringFromMap(paramMap, "validation"),
						}
						skill.Parameters[name] = param
					}
				}
			}
		}
	}

	return skill, nil
}

// extractDomainFromName extracts domain from skill name
func extractDomainFromName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		switch parts[0] {
		case "github":
			return "github"
		case "weather":
			return "weather"
		case "web":
			return "web"
		case "search":
			return "search"
		case "extract":
			return "extract"
		case "email":
			return "email"
		case "slack":
			return "slack"
		default:
			return "general"
		}
	}
	return "general"
}

// getRiskForDomain returns risk level for domain
func getRiskForDomain(domain string) string {
	switch domain {
	case "weather":
		return "low"
	case "github":
		return "medium"
	case "web":
		return "medium"
	case "search":
		return "medium"
	case "extract":
		return "medium"
	case "email":
		return "high"
	case "slack":
		return "medium"
	default:
		return "low"
	}
}

// getCommandForSkill returns base command for skill
func getCommandForSkill(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		switch parts[0] {
		case "weather":
			return "curl"
		case "github":
			return "gh"
		case "web":
			return "curl"
		case "search":
			return "curl"
		case "extract":
			return "curl"
		case "email":
			return "curl"
		case "slack":
			return "curl"
		}
	}
	return "curl" // default
}

// GetByDomain returns all skills for a given domain
func (r *Registry) GetByDomain(domain string) []*Skill {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	skills := r.domainIndex[domain]
	result := make([]*Skill, len(skills))
	copy(result, skills)
	return result
}

// GetEnabled returns all enabled skills
func (r *Registry) GetEnabled() []*Skill {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*Skill
	for _, skill := range r.skills {
		if skill.Enabled {
			result = append(result, skill)
		}
	}
	return result
}

// GetSkill returns a specific skill by name
func (r *Registry) GetSkill(name string) (*Skill, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	skill, exists := r.skills[name]
	return skill, exists
}

// SetEnabled enables or disables a skill
func (r *Registry) SetEnabled(name string, enabled bool) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	skill, exists := r.skills[name]
	if !exists {
		return false
	}

	skill.Enabled = enabled
	return true
}

// convertInterfaceMap converts interface maps to map[string]interface{}
func convertInterfaceMap(m interface{}) map[string]interface{} {
	switch v := m.(type) {
	case map[string]interface{}:
		return v
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[fmt.Sprintf("%v", key)] = val
		}
		return result
	default:
		return make(map[string]interface{})
	}
}

// parseYAMLFrontmatter parses YAML frontmatter using yaml.v3
func parseYAMLFrontmatter(data string, v interface{}) error {
	return yaml.Unmarshal([]byte(data), v)
}

func getStringFromMap(m map[string]interface{}, key string) string {
	val, _ := m[key].(string)
	return val
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	val, ok := m[key]
	if !ok {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return strings.ToLower(v) == "true"
	}
	return false
}

// RegisterWebDAV registers the WebDAV skill programmatically
func RegisterWebDAV(r *Registry) {
	skill := &Skill{
		Name:        "webdav",
		Description: "WebDAV protocol operations (list, get, put, delete) for accessing WebDAV servers",
		Homepage:    "https://tools.ietf.org/html/rfc4918",
		Domain:      "webdav",
		Risk:        "medium",
		Command:     "http",
		Enabled:     true,
		Timeout:     60 * time.Second,
		Version:     "1.0.0",
		Parameters: map[string]Param{
			"url": {
				Type:        "string",
				Description: "WebDAV server URL (required)",
				Required:    true,
			},
			"operation": {
				Type:        "string",
				Description: "Operation: list, get, put, or delete (required)",
				Required:    true,
			},
			"username": {
				Type:        "string",
				Description: "Username for authentication (optional)",
				Required:    false,
			},
			"password": {
				Type:        "string",
				Description: "Password for authentication (optional)",
				Required:    false,
			},
			"content": {
				Type:        "array",
				Description: "Content bytes for PUT operation (required for put)",
				Required:    false,
			},
			"content_length": {
				Type:        "number",
				Description: "Content length for PUT operation (required for put)",
				Required:    false,
			},
			"content_type": {
				Type:        "string",
				Description: "Content type header for PUT operation (optional)",
				Required:    false,
			},
		},
		Metadata: map[string]interface{}{
			"protocol":      "WebDAV",
			"rfc":           "4918",
			"methods":       []string{"PROPFIND", "GET", "PUT", "DELETE"},
			"ssl_required":  true,
			"auth_methods":  []string{"Basic"},
			"content_types": []string{"application/xml", "multipart/form-data"},
		},
	}

	r.mutex.Lock()
	r.skills[skill.Name] = skill
	r.domainIndex[skill.Domain] = append(r.domainIndex[skill.Domain], skill)
	r.mutex.Unlock()
}

// RegisterCalendar registers the Calendar skill programmatically
func RegisterCalendar(r *Registry) {
	skill := &Skill{
		Name:        "calendar",
		Description: "CalDAV-based calendar operations (list, create, get, update, delete events) for accessing calendar servers",
		Homepage:    "https://tools.ietf.org/html/rfc4791",
		Domain:      "calendar",
		Risk:        "medium",
		Command:     "curl",
		Enabled:     true,
		Timeout:     60 * time.Second,
		Version:     "1.0.0",
		Parameters: map[string]Param{
			"operation": {
				Type:        "string",
				Description: "Operation: list_calendars, create_event, get_events, get_event, update_event, or delete_event (required)",
				Required:    true,
			},
			"calendar_url": {
				Type:        "string",
				Description: "CalDAV server URL (required)",
				Required:    true,
			},
			"username": {
				Type:        "string",
				Description: "Username for authentication (optional)",
				Required:    false,
			},
			"password": {
				Type:        "string",
				Description: "Password for authentication (optional)",
				Required:    false,
			},
			"title": {
				Type:        "string",
				Description: "Event title (required for create_event)",
				Required:    false,
			},
			"description": {
				Type:        "string",
				Description: "Event description (optional)",
				Required:    false,
			},
			"location": {
				Type:        "string",
				Description: "Event location (optional)",
				Required:    false,
			},
			"attendees": {
				Type:        "array",
				Description: "List of attendee email addresses (optional)",
				Required:    false,
			},
			"start_time": {
				Type:        "string",
				Description: "Event start time in RFC3339 or '2006-01-02 15:04:05' format (required for create_event/update_event)",
				Required:    false,
			},
			"end_time": {
				Type:        "string",
				Description: "Event end time in RFC3339 or '2006-01-02 15:04:05' format (required for create_event/update_event)",
				Required:    false,
			},
			"event_data": {
				Type:        "object",
				Description: "Event data object for get_event, update_event, delete_event (required for these operations)",
				Required:    false,
			},
		},
		Metadata: map[string]interface{}{
			"protocol":           "CalDAV",
			"rfc":                "4791",
			"operations":         []string{"list_calendars", "create_event", "get_events", "get_event", "update_event", "delete_event"},
			"ssl_required":       true,
			"auth_methods":       []string{"Basic"},
			"conflict_detection": true,
		},
	}

	r.mutex.Lock()
	r.skills[skill.Name] = skill
	r.domainIndex[skill.Domain] = append(r.domainIndex[skill.Domain], skill)
	r.mutex.Unlock()
}
