// Package navchart provides tab-scoped NavChart storage, replay,
// and diagnostics diff for browser automation sessions.
package navchart

// ActionType identifies the kind of browser action recorded in a chart.
type ActionType string

const (
	ActionClick    ActionType = "click"
	ActionInput    ActionType = "input"
	ActionNavigate ActionType = "navigate"
	ActionWait     ActionType = "wait"
	ActionAssert   ActionType = "assert"
)

// SelectorTier identifies which selector tier succeeded during replay.
type SelectorTier string

const (
	TierPrimary   SelectorTier = "primary"
	TierSecondary SelectorTier = "secondary"
	TierFallback  SelectorTier = "fallback"
	TierFailed    SelectorTier = "failed"
)

// NavChart represents a recorded browser navigation chart.
// TabID is empty for global/single-tab mode; populated when
// FEATURE_MULTI_TAB_REPLAY is enabled.
type NavChart struct {
	Version      int                    `json:"version"`
	TargetDomain string                 `json:"target_domain"`
	TabID        string                 `json:"tab_id,omitempty"`
	Metadata     ChartMetadata          `json:"metadata"`
	ActionMap    map[string]ChartAction `json:"action_map"`
}

// ChartMetadata holds generation context for a NavChart.
type ChartMetadata struct {
	GeneratedBy string `json:"generated_by"`
	Timestamp   string `json:"timestamp"`
	SessionID   string `json:"session_id,omitempty"`
}

// ChartAction describes a single browser action within a NavChart.
type ChartAction struct {
	ActionType     ActionType     `json:"action_type"`
	Selector       *ChartSelector `json:"selector,omitempty"`
	Value          string         `json:"value,omitempty"`
	URL            string         `json:"url,omitempty"`
	FrameRouting   *FrameRouting  `json:"frame_routing,omitempty"`
	PostActionWait *WaitCondition `json:"post_action_wait,omitempty"`
	Assertion      *Assertion     `json:"assertion,omitempty"`
}

// ChartSelector holds a 3-tier selector fallback matrix.
type ChartSelector struct {
	PrimaryCSS     string `json:"primary_css"`
	SecondaryXPath string `json:"secondary_xpath,omitempty"`
	FallbackJS     string `json:"fallback_js,omitempty"`
}

// FrameRouting describes iframe targeting for an action.
type FrameRouting struct {
	Selector string `json:"selector,omitempty"`
	Name     string `json:"name,omitempty"`
	Origin   string `json:"origin,omitempty"`
}

// WaitCondition describes a post-action wait requirement.
type WaitCondition struct {
	Type     string         `json:"type"`
	Selector *ChartSelector `json:"selector,omitempty"`
	Timeout  int            `json:"timeout,omitempty"`
}

// Assertion describes an expected state after an action.
type Assertion struct {
	Type     string      `json:"type"`
	Expected interface{} `json:"expected"`
}
