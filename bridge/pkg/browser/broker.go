package browser

// Deprecated: The legacy (Playwright-based) browser backend is deprecated but
// still supported as a fallback. Use the Jetski (CDP proxy) backend instead
// by setting ARMORCLAW_BROWSER_BACKEND=jetski. Legacy support will be removed
// in a future version.

import "context"

// BrowserBroker manages browser automation jobs and exposes semantic
// operations consumed by the handler layer and queue subsystem.
type BrowserBroker interface {
	StartJob(ctx context.Context, req StartJobRequest) (JobID, error)
	Status(ctx context.Context, id JobID) (*JobSummary, error)
	Complete(ctx context.Context, id JobID) (*BrokerResult, error)
	Fail(ctx context.Context, id JobID, reason string) error
	List(ctx context.Context, agentID string) ([]JobSummary, error)
	Cancel(ctx context.Context, id JobID) error

	Navigate(ctx context.Context, id JobID, url string) (*BrokerResult, error)

	// Fill injects values into form fields. When a field's Sensitive flag
	// is true, the broker routes it through the PII approval path before
	// injecting via ServiceFillField.ValueRef.
	Fill(ctx context.Context, id JobID, fields []FillRequest) (*BrokerResult, error)

	Click(ctx context.Context, id JobID, selector string) (*BrokerResult, error)
	WaitForElement(ctx context.Context, id JobID, selector string, timeoutMs int) (*BrokerResult, error)
	WaitForCaptcha(ctx context.Context, id JobID, timeoutMs int) (*BrokerResult, error)
	WaitFor2FA(ctx context.Context, id JobID, timeoutMs int) (*BrokerResult, error)
	Extract(ctx context.Context, id JobID, spec ExtractSpec) (*ExtractResult, error)
	Screenshot(ctx context.Context, id JobID, fullPage bool) (*BrokerResult, error)

	// ReplayChart replays all actions from a NavChart. Sensitive input actions
	// (PII placeholders) are routed through the Bridge PII approval path.
	ReplayChart(ctx context.Context, jobID JobID, chart NavChart, piiValues map[string]string) error
}
