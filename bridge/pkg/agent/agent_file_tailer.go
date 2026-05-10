// Package agent provides agent management functionality for ArmorClaw.
//
// agent_file_tailer.go implements the bridge-side reader for agent backward
// communication files (agent_status.json and agent_events.jsonl). It polls
// the bind-mounted state directory every 500ms — the same pattern as
// secretary.EventReader — and delivers parsed results via callbacks.
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/armorclaw/bridge/internal/events"
)

const (
	// pollInterval is the interval between file polls (matches EventReader pattern).
	pollInterval = 500 * time.Millisecond

	// maxEventFileSize is the soft cap for the events JSONL file.
	// When exceeded the tailer stops reading but the agent continues normally.
	maxEventFileSize = 10 * 1024 * 1024 // 10 MB

	// pipeBufSize is the Linux PIPE_BUF limit. Lines exceeding this are
	// logged as warnings and skipped to avoid partial-read corruption.
	pipeBufSize = 4096

	// agentEventsFile is the JSONL file appended by the agent container.
	agentEventsFile = "agent_events.jsonl"

	// agentStatusFile is the atomic-rename status file written by the container.
	agentStatusFile = "agent_status.json"
)

// ErrEventLogExceeded indicates the events file exceeded the 10 MB soft cap.
var ErrEventLogExceeded = errors.New("agent event log exceeded 10MB cap")

// AgentFileEvent mirrors one JSON line from agent_events.jsonl.
// Schema matches doc/agent-file-protocol.md File 2.
type AgentFileEvent struct {
	Seq        int                    `json:"seq"`
	Type       string                 `json:"type"`
	Name       string                 `json:"name"`
	TsMs       int64                  `json:"ts_ms"`
	Detail     map[string]interface{} `json:"detail,omitempty"`
	DurationMs *int                   `json:"duration_ms,omitempty"`
}

// AgentFileStatus mirrors the JSON object from agent_status.json.
// Schema matches doc/agent-file-protocol.md File 1.
type AgentFileStatus struct {
	AgentID   string         `json:"agent_id"`
	State     string         `json:"state"`
	Timestamp int64          `json:"timestamp"`
	Message   string         `json:"message,omitempty"`
	Metadata  StatusMetadata `json:"metadata,omitempty"`
}

// AgentFileTailer polls agent_status.json and agent_events.jsonl in a
// bind-mounted state directory, delivering parsed results through callbacks.
//
// It uses the same polling approach as secretary.EventReader: 500 ms
// interval, incremental byte-offset tracking for the JSONL file, and a
// 10 MB soft cap. Polling (not inotify) ensures compatibility with
// bind-mounted directories across filesystem boundaries.
type AgentFileTailer struct {
	stateDir string

	// Callbacks — must not be nil when Start is called.
	onEvent  func(AgentFileEvent)
	onStatus func(AgentFileStatus)

	// Optional Matrix event bus for forwarding agent events to Matrix rooms.
	eventBus *events.MatrixEventBus
	roomID   string
	agentID  string

	// Events file offset tracking.
	byteOffset int64
	lastSeq    int

	// Status file dedup — only invoke onStatus when content changes.
	lastStatusJSON string

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

// NewAgentFileTailer creates a tailer that watches stateDir for agent files.
//
//   - onEvent:  called for each new line parsed from agent_events.jsonl
//   - onStatus: called whenever agent_status.json content changes
func NewAgentFileTailer(stateDir string, onEvent func(AgentFileEvent), onStatus func(AgentFileStatus)) *AgentFileTailer {
	return &AgentFileTailer{
		stateDir: stateDir,
		onEvent:  onEvent,
		onStatus: onStatus,
	}
}

// WithEventBus configures the tailer to forward agent events and status
// changes to the MatrixEventBus. This is additive: callbacks are still
// invoked. Emission failures are logged as warnings and never block the
// tailer's polling loop.
func (t *AgentFileTailer) WithEventBus(bus *events.MatrixEventBus, roomID, agentID string) *AgentFileTailer {
	t.eventBus = bus
	t.roomID = roomID
	t.agentID = agentID
	return t
}

// Start begins the polling goroutine. It runs until ctx is cancelled or
// Stop is called.
func (t *AgentFileTailer) Start(ctx context.Context) {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.running = true
	ctx, t.cancel = context.WithCancel(ctx)
	t.mu.Unlock()

	go t.run(ctx)
}

// Stop gracefully shuts down the polling goroutine.
func (t *AgentFileTailer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
	t.running = false
}

// emitEventToBus forwards an AgentFileEvent to the MatrixEventBus,
// mapping agent event types to existing workflow event types so the
// MatrixEventForwarder can relay them as m.notice messages.
func (t *AgentFileTailer) emitEventToBus(evt AgentFileEvent) {
	if t.eventBus == nil || t.roomID == "" {
		return
	}

	eventType := "workflow.step_progress"
	switch evt.Type {
	case "error":
		eventType = "workflow.step_error"
	case "blocker":
		eventType = "workflow.blocker_warning"
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("agent_file_tailer: panic emitting event to bus: %v", r)
			}
		}()
		t.eventBus.Publish(events.MatrixEvent{
			ID:      fmt.Sprintf("agent-evt-%s-%d-%d", t.agentID, evt.Seq, time.Now().UnixNano()),
			RoomID:  t.roomID,
			Sender:  "agent-tailer",
			Type:    eventType,
			Content: evt,
		})
	}()
}

// emitStatusToBus forwards an AgentFileStatus change to the MatrixEventBus
// using the canonical agent status event type.
func (t *AgentFileTailer) emitStatusToBus(status AgentFileStatus) {
	if t.eventBus == nil || t.roomID == "" {
		return
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("agent_file_tailer: panic emitting status to bus: %v", r)
			}
		}()
		t.eventBus.Publish(events.MatrixEvent{
			ID:      fmt.Sprintf("agent-status-%s-%d", t.agentID, time.Now().UnixNano()),
			RoomID:  t.roomID,
			Sender:  "agent-tailer",
			Type:    "com.armorclaw.agent.status",
			Content: status,
		})
	}()
}

// run is the main polling loop.
func (t *AgentFileTailer) run(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.pollStatus()
			if err := t.pollEvents(); err != nil {
				if errors.Is(err, ErrEventLogExceeded) {
					log.Printf("agent_file_tailer: events file exceeded 10MB cap in %s, stopping tail", t.stateDir)
					t.mu.Lock()
					t.running = false
					t.mu.Unlock()
					return
				}
				log.Printf("agent_file_tailer: error polling events: %v", err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Status polling
// ---------------------------------------------------------------------------

// pollStatus reads agent_status.json if it exists and invokes onStatus
// when the content differs from the previous read.
func (t *AgentFileTailer) pollStatus() {
	path := filepath.Join(t.stateDir, agentStatusFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("agent_file_tailer: read status: %v", err)
		}
		return
	}

	content := string(data)

	t.mu.Lock()
	if content == t.lastStatusJSON {
		t.mu.Unlock()
		return
	}
	t.lastStatusJSON = content
	t.mu.Unlock()

	var status AgentFileStatus
	if err := json.Unmarshal(data, &status); err != nil {
		log.Printf("agent_file_tailer: parse status JSON: %v", err)
		return
	}

	t.onStatus(status)
	t.emitStatusToBus(status)
}

// ---------------------------------------------------------------------------
// Events polling (incremental tail)
// ---------------------------------------------------------------------------

// pollEvents reads new lines appended to agent_events.jsonl since the last
// call, invoking onEvent for each valid parsed event.
func (t *AgentFileTailer) pollEvents() error {
	path := filepath.Join(t.stateDir, agentEventsFile)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat events file: %w", err)
	}

	if info.Size() > maxEventFileSize {
		return ErrEventLogExceeded
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	if _, err := f.Seek(t.byteOffset, io.SeekStart); err != nil {
		return fmt.Errorf("seek events file: %w", err)
	}

	scanner := bufio.NewScanner(f)
	// Allow up to 64KB per line for the scanner buffer, but we validate
	// PIPE_BUF on each actual line.
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventFileSize+1)

	for scanner.Scan() {
		raw := scanner.Text()

		// Advance offset: line bytes + newline.
		t.byteOffset += int64(len(raw)) + 1

		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// PIPE_BUF validation: skip oversized lines with a warning.
		if len(line) > pipeBufSize {
			log.Printf("agent_file_tailer: skipping oversized line (%d bytes, limit %d): %.80s...",
				len(line), pipeBufSize, line)
			continue
		}

		var evt AgentFileEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			log.Printf("agent_file_tailer: skipping malformed JSON line: %q: %v", line, err)
			continue
		}

		// Deduplicate by sequence number.
		if evt.Seq <= t.lastSeq {
			continue
		}
		t.lastSeq = evt.Seq

		t.onEvent(evt)
		t.emitEventToBus(evt)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan events file: %w", err)
	}

	return nil
}
