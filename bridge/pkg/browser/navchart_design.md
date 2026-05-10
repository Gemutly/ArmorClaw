# NavChartStore Interface and Caller Audit

> **Status**: BLOCKING GATE — T6 implementation cannot proceed until reviewed.
> **Date**: 2026-05-09
> **Branch**: stabilization-v4

## 1. NavChartStore Field Type

**Location**: `bridge/pkg/rpc/server.go:241`

```go
NavChartStore    *navchart.MultiTabStore
```

**It is a concrete type**, NOT an interface. The full qualified type is `*github.com/armorclaw/jetski/navchart.MultiTabStore`.

**Current wiring status**: `bridge/cmd/bridge/main.go` — wired at startup:
```go
navChartStore := navchart.NewMultiTabStore()
rpcCfg.NavChartStore = navChartStore
```

---

## 2. MultiTabStore Struct and Methods

**Location**: `jetski/navchart/multi_tab.go`

```go
type MultiTabStore struct {
    mu     sync.RWMutex
    charts map[string][]NavChart
}
```

### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewMultiTabStore` | `func NewMultiTabStore() *MultiTabStore` | Constructor. Creates initialized empty store. |
| `StoreChart` | `func (s *MultiTabStore) StoreChart(tabID string, chart NavChart) error` | Appends a NavChart for the given tabID. Sets chart.TabID. Returns error if tabID is empty. |
| `GetCharts` | `func (s *MultiTabStore) GetCharts(tabID string) ([]NavChart, error)` | Returns all NavCharts for tabID. Returns empty slice (not nil) if no charts. Returns a defensive copy. |
| `RemoveTab` | `func (s *MultiTabStore) RemoveTab(tabID string) error` | Deletes all charts for tabID. Safe to call for non-existent tab. |
| `TabIDs` | `func (s *MultiTabStore) TabIDs() []string` | Returns all tab IDs that have stored charts. |

**Storage**: In-memory only (`map[string][]NavChart` with sync.RWMutex). No SQLite persistence. No disk backing. Data is lost on restart.

---

## 3. NavChart Data Types

**Two duplicate type definitions exist**:

### 3a. `jetski/navchart/types.go` (authoritative — imported by rpc/server.go)

```go
type NavChart struct {
    Version      int                    `json:"version"`
    TargetDomain string                 `json:"target_domain"`
    TabID        string                 `json:"tab_id,omitempty"`
    Metadata     ChartMetadata          `json:"metadata"`
    ActionMap    map[string]ChartAction `json:"action_map"`
}

type ChartMetadata struct {
    GeneratedBy string `json:"generated_by"`
    Timestamp   string `json:"timestamp"`
    SessionID   string `json:"session_id,omitempty"`
}

type ChartAction struct {
    ActionType     ActionType     `json:"action_type"`
    Selector       *ChartSelector `json:"selector,omitempty"`
    Value          string         `json:"value,omitempty"`
    URL            string         `json:"url,omitempty"`
    FrameRouting   *FrameRouting  `json:"frame_routing,omitempty"`
    PostActionWait *WaitCondition `json:"post_action_wait,omitempty"`
    Assertion      *Assertion     `json:"assertion,omitempty"`
}

type ChartSelector struct {
    PrimaryCSS     string `json:"primary_css"`
    SecondaryXPath string `json:"secondary_xpath,omitempty"`
    FallbackJS     string `json:"fallback_js,omitempty"`
}

type FrameRouting struct {
    Selector string `json:"selector,omitempty"`
    Name     string `json:"name,omitempty"`
    Origin   string `json:"origin,omitempty"`
}

type WaitCondition struct {
    Type     string         `json:"type"`
    Selector *ChartSelector `json:"selector,omitempty"`
    Timeout  int            `json:"timeout,omitempty"`
}

type Assertion struct {
    Type     string      `json:"type"`
    Expected interface{} `json:"expected"`
}
```

### 3b. `bridge/pkg/browser/chart_types.go` (mirror for browser package)

Identical structure — same field names, same JSON tags, same types. Used by `BrowserBroker.ReplayChart` and `JetskiBroker`. These are **package-local duplicates**, not shared imports.

### 3c. Supporting types in navchart package

```go
type ActionType string  // "click" | "input" | "navigate" | "wait" | "assert"
type SelectorTier string // "primary" | "secondary" | "fallback" | "failed"

type ChartDiff struct {
    Action   string `json:"action"`
    Expected string `json:"expected"`
    Actual   string `json:"actual"`
    Match    bool   `json:"match"`
}

type ReplayConfig struct {
    TabID          string
    MultiTabReplay bool
    Chart          NavChart
}

type ReplayResult struct {
    TabID    string
    Actions  int
    Success  bool
    Diffs    []ChartDiff
    Err      error
}
```

---

## 4. All Callers of NavChartStore Methods

### 4a. `browser.replay_diagnostics` RPC Handler

**Location**: `bridge/pkg/rpc/browser.go:950-1032` (`handleBrowserReplayDiagnostics`)

**Method calls on `s.navChartStore`**:

1. `s.navChartStore.GetCharts(params.TabID)` — retrieves all stored charts for a tab
2. **Indirect**: `navchart.DiffCharts(expected, actual)` — called with chart data from GetCharts

**Flow**:
1. Checks `s.replayFlags.MultiTabReplay` — returns `-32601` if disabled
2. Validates `tab_id` param is non-empty
3. Checks `s.navChartStore != nil` — returns `-32603` if nil
4. Calls `GetCharts(tabID)` to retrieve stored charts
5. If ≥2 charts exist, diffs last two using `navchart.DiffCharts`
6. Returns `{tab_id, diffs, match_percentage}`

**Test coverage**: `bridge/pkg/rpc/replay_diagnostics_test.go`, `bridge/pkg/rpc/replay_gating_test.go`, `bridge/pkg/rpc/edge_case_test.go`

### 4b. JetskiBroker (NavChart Consumer)

**Location**: `bridge/pkg/browser/jetski_broker.go:1201`

**Method**: `ReplayChart(ctx context.Context, jobID JobID, chart NavChart, piiValues map[string]string) error`

**Does NOT call NavChartStore directly.** It receives a `NavChart` as a parameter and replays it action-by-action. The store is not involved in replay execution — it's only used by the RPC diagnostics handler.

However, `JetskiBroker` is the **only producer** of NavChart-related browser operations. The `BrowserBroker` interface declares:
```go
ReplayChart(ctx context.Context, jobID JobID, chart NavChart, piiValues map[string]string) error
```

### 4c. Test files constructing NavChartStore

All test files use `navchart.NewMultiTabStore()` directly:

| File | Lines | Usage |
|------|-------|-------|
| `bridge/pkg/rpc/replay_diagnostics_test.go` | 42, 65, 88, 139 | Creates store, injects into `Server{navChartStore: store}` |
| `bridge/pkg/rpc/replay_gating_test.go` | 16 | Creates store for flag-gating tests |
| `bridge/pkg/rpc/edge_case_test.go` | 433, 455, 476, 496, 695, 752 | Edge case tests with store |

---

## 5. NavChart Package Utility Functions

**Location**: `jetski/navchart/diagnostics.go`

| Function | Signature | Description |
|----------|-----------|-------------|
| `DiffCharts` | `func DiffCharts(expected, actual NavChart) []ChartDiff` | Compares two NavCharts action-by-action in sorted key order |
| `DiffReplay` | `func DiffReplay(tabID string, store *MultiTabStore, replayed NavChart) ([]ChartDiff, error)` | Gets latest chart from store and diffs against replayed chart |
| `actionSummary` | `func actionSummary(a ChartAction) string` | (unexported) Human-readable one-line summary |

**Location**: `jetski/navchart/replay.go`

| Function | Signature | Description |
|----------|-----------|-------------|
| `Replay` | `func Replay(cfg ReplayConfig) (*ReplayResult, error)` | Validates replay config, returns ready result |
| `ReplayWithStore` | `func ReplayWithStore(cfg ReplayConfig, store *MultiTabStore) (*ReplayResult, error)` | Validates + diffs against store |

---

## 6. Gaps and Observations for T6 Implementation

### Current State
- `NavChartStore` is a **concrete type** (`*navchart.MultiTabStore`), not an interface
- It is **always nil** — never wired in `main.go`
- Storage is **in-memory only** — no SQLite, no disk persistence
- No `StoreChart()` call exists anywhere in production code — only in tests
- The `replay_diagnostics` RPC handler is the **sole consumer** of `navChartStore.GetCharts()`

### What T6 Needs to Address
1. **Wire construction**: `NewMultiTabStore()` must be called in main.go and injected into rpc.Config
2. **Persistence**: Plan mentions SQLite persistence — current MultiTabStore is in-memory only
3. **StoreChart callers**: No production code currently calls `StoreChart()` — needs a producer
4. **PII scan**: Plan mentions PII scanning — no PII scanning exists in current code
5. **LRU eviction**: Plan mentions LRU eviction — no eviction logic exists
6. **AuditLog integration**: Plan mentions audit logging — no audit integration exists
7. **Duplicate types**: `browser.ChartAction` and `navchart.ChartAction` are identical duplicates — may need unification

### Interface vs Concrete Decision
Since the type is `*navchart.MultiTabStore` (concrete pointer), T6 can either:
- **Option A**: Keep concrete, add methods to MultiTabStore (persistence, PII scan, LRU, audit)
- **Option B**: Extract an interface from MultiTabStore's public methods, make rpc.Server depend on the interface, implement SQLite-backed store behind it

The current method surface is small (4 methods: `StoreChart`, `GetCharts`, `RemoveTab`, `TabIDs`), making either approach viable.
