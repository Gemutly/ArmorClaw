package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStatus(t *testing.T, dir string, status AgentFileStatus) {
	t.Helper()
	data, err := json.Marshal(status)
	require.NoError(t, err)
	tmp := filepath.Join(dir, "agent_status.json.tmp")
	require.NoError(t, os.WriteFile(tmp, data, 0644))
	require.NoError(t, os.Rename(tmp, filepath.Join(dir, agentStatusFile)))
}

func appendEvent(t *testing.T, dir, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, agentEventsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = fmt.Fprintln(f, line)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestAgentFileTailer_PicksUpEventsWithinOneSecond(t *testing.T) {
	dir := t.TempDir()

	var events []AgentFileEvent
	var mu syncMutex
	tailer := NewAgentFileTailer(dir, func(e AgentFileEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}, func(_ AgentFileStatus) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tailer.Start(ctx)

	appendEvent(t, dir, `{"seq":1,"type":"step","name":"start","ts_ms":0}`)
	appendEvent(t, dir, `{"seq":2,"type":"step","name":"middle","ts_ms":100}`)

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) >= 2
	}, 2*time.Second, 100*time.Millisecond, "expected at least 2 events within 2s")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, len(events))
	assert.Equal(t, 1, events[0].Seq)
	assert.Equal(t, "start", events[0].Name)
	assert.Equal(t, 2, events[1].Seq)
	assert.Equal(t, "middle", events[1].Name)
}

func TestAgentFileTailer_StopsAt10MBCap(t *testing.T) {
	dir := t.TempDir()

	var capReached atomic.Bool
	tailer := NewAgentFileTailer(dir, func(_ AgentFileEvent) {}, func(_ AgentFileStatus) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tailer.Start(ctx)

	eventsPath := filepath.Join(dir, agentEventsFile)
	f, err := os.Create(eventsPath)
	require.NoError(t, err)

	chunk := strings.Repeat("x", 4096)
	for i := 0; i < 2600; i++ {
		fmt.Fprintln(f, chunk)
	}
	require.NoError(t, f.Close())

	assert.Eventually(t, func() bool {
		info, err := os.Stat(eventsPath)
		if err != nil {
			return false
		}
		return info.Size() > maxEventFileSize
	}, 2*time.Second, 100*time.Millisecond, "events file should exceed 10MB")

	assert.Eventually(t, func() bool {
		return !tailer.running
	}, 3*time.Second, 200*time.Millisecond, "tailer should stop running after cap reached")

	assert.True(t, capReached.Load() || !tailer.running, "tailer should have stopped")
}

func TestAgentFileTailer_SkipsOversizedLines(t *testing.T) {
	dir := t.TempDir()

	var events []AgentFileEvent
	var mu syncMutex
	tailer := NewAgentFileTailer(dir, func(e AgentFileEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}, func(_ AgentFileStatus) {})

	oversized := strings.Repeat("a", 5000)
	appendEvent(t, dir, oversized)
	appendEvent(t, dir, `{"seq":1,"type":"step","name":"good","ts_ms":0}`)

	err := tailer.pollEvents()
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, events, 1)
	assert.Equal(t, "good", events[0].Name)
}

func TestAgentFileTailer_ReadsStatusChanges(t *testing.T) {
	dir := t.TempDir()

	var statuses []AgentFileStatus
	var mu syncMutex
	tailer := NewAgentFileTailer(dir, func(_ AgentFileEvent) {}, func(s AgentFileStatus) {
		mu.Lock()
		statuses = append(statuses, s)
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tailer.Start(ctx)

	writeStatus(t, dir, AgentFileStatus{
		AgentID:   "agent-1",
		State:     "INITIALIZING",
		Timestamp: 1000,
	})

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(statuses) >= 1
	}, 2*time.Second, 100*time.Millisecond)

	mu.Lock()
	require.Len(t, statuses, 1)
	assert.Equal(t, "agent-1", statuses[0].AgentID)
	assert.Equal(t, "INITIALIZING", statuses[0].State)
	mu.Unlock()

	writeStatus(t, dir, AgentFileStatus{
		AgentID:   "agent-1",
		State:     "BROWSING",
		Timestamp: 2000,
		Message:   "Loading page",
	})

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(statuses) >= 2
	}, 2*time.Second, 100*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, statuses, 2)
	assert.Equal(t, "BROWSING", statuses[1].State)
	assert.Equal(t, "Loading page", statuses[1].Message)
}

func TestAgentFileTailer_DeduplicatesStatus(t *testing.T) {
	dir := t.TempDir()

	var count atomic.Int32
	tailer := NewAgentFileTailer(dir, func(_ AgentFileEvent) {}, func(_ AgentFileStatus) {
		count.Add(1)
	})

	status := AgentFileStatus{AgentID: "a1", State: "IDLE", Timestamp: 1000}

	tailer.pollStatus()
	assert.Equal(t, int32(0), count.Load(), "no file yet = no callback")

	writeStatus(t, dir, status)
	tailer.pollStatus()
	assert.Equal(t, int32(1), count.Load(), "first write triggers callback")

	tailer.pollStatus()
	assert.Equal(t, int32(1), count.Load(), "same content = no callback")
}

func TestAgentFileTailer_SkipsCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()

	var events []AgentFileEvent
	var mu syncMutex
	tailer := NewAgentFileTailer(dir, func(e AgentFileEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}, func(_ AgentFileStatus) {})

	appendEvent(t, dir, "# this is a comment")
	appendEvent(t, dir, "")
	appendEvent(t, dir, "   ")
	appendEvent(t, dir, `{"seq":1,"type":"step","name":"after-blanks","ts_ms":0}`)

	err := tailer.pollEvents()
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, events, 1)
	assert.Equal(t, "after-blanks", events[0].Name)
}

func TestAgentFileTailer_DeduplicatesBySeq(t *testing.T) {
	dir := t.TempDir()

	var events []AgentFileEvent
	var mu syncMutex
	tailer := NewAgentFileTailer(dir, func(e AgentFileEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}, func(_ AgentFileStatus) {})

	appendEvent(t, dir, `{"seq":1,"type":"step","name":"first","ts_ms":0}`)
	require.NoError(t, tailer.pollEvents())

	appendEvent(t, dir, `{"seq":1,"type":"step","name":"first-dup","ts_ms":0}`)
	appendEvent(t, dir, `{"seq":2,"type":"step","name":"second","ts_ms":100}`)
	require.NoError(t, tailer.pollEvents())

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, events, 2)
	assert.Equal(t, "first", events[0].Name)
	assert.Equal(t, "second", events[1].Name)
}

func TestAgentFileTailer_MissingDirIsNoError(t *testing.T) {
	dir := t.TempDir()
	tailer := NewAgentFileTailer(dir, func(_ AgentFileEvent) {}, func(_ AgentFileStatus) {})

	require.NoError(t, tailer.pollEvents())
	tailer.pollStatus()
}

func TestAgentFileTailer_StopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	tailer := NewAgentFileTailer(dir, func(_ AgentFileEvent) {}, func(_ AgentFileStatus) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tailer.Start(ctx)

	tailer.Stop()
	tailer.Stop()
}

// syncMutex avoids collision with the sync package import.
type syncMutex struct {
	lock chan struct{}
}

func (m *syncMutex) Lock()   { m.getChan() <- struct{}{} }
func (m *syncMutex) Unlock() { <-m.getChan() }

func (m *syncMutex) getChan() chan struct{} {
	if m.lock == nil {
		m.lock = make(chan struct{}, 1)
	}
	return m.lock
}
