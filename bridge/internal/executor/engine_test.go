package executor

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mock SkillRegistry
// ---------------------------------------------------------------------------

type mockSkillRegistry struct {
	getSkillFunc func(name string) (*Skill, bool)
}

func (m *mockSkillRegistry) GetSkill(name string) (*Skill, bool) {
	if m.getSkillFunc != nil {
		return m.getSkillFunc(name)
	}
	return nil, false
}

func shellRegistry() *mockSkillRegistry {
	return &mockSkillRegistry{
		getSkillFunc: func(name string) (*Skill, bool) {
			if name == "shell" {
				return &Skill{Name: "shell", Command: "shell", Timeout: 5 * time.Second}, true
			}
			return nil, false
		},
	}
}

// ---------------------------------------------------------------------------
// 1. TestNewToolExecutor — verify defaults
// ---------------------------------------------------------------------------

func TestNewToolExecutor(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	assert.NotNil(t, te)
	assert.Equal(t, 30*time.Second, te.timeout, "default timeout should be 30s")
	assert.NotNil(t, te.pool, "pool should be initialised")
	assert.NotNil(t, te.skills, "skills registry should be set")
}

func TestNewToolExecutor_ZeroValues(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{})
	defer te.Close()

	assert.Equal(t, 30*time.Second, te.timeout, "zero timeout → default 30s")
	assert.NotNil(t, te.pool)
}

func TestNewToolExecutor_CustomValues(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       5 * time.Second,
		MaxWorkers:    4,
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	assert.Equal(t, 5*time.Second, te.timeout)
}

// ---------------------------------------------------------------------------
// 2. TestExecute_DirectSkill — shell echo through pool
// ---------------------------------------------------------------------------

func TestExecute_DirectSkill(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       5 * time.Second,
		MaxWorkers:    2,
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	result, err := te.Execute(context.Background(), ToolCall{
		ID:   "tc-1",
		Name: "shell",
		Args: map[string]interface{}{"command": "echo hello"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "tc-1", result.CallID)
	assert.Equal(t, "shell", result.Name)
	assert.Contains(t, result.Output, "hello")
	assert.NoError(t, result.Error)
}

// ---------------------------------------------------------------------------
// 3. TestExecute_UnknownSkill_Error
// ---------------------------------------------------------------------------

func TestExecute_UnknownSkill_Error(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       5 * time.Second,
		MaxWorkers:    2,
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	_, err := te.Execute(context.Background(), ToolCall{
		ID:   "tc-2",
		Name: "nonexistent",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

// ---------------------------------------------------------------------------
// 4. TestExecute_NilRegistry_SkipsCheck
// ---------------------------------------------------------------------------

func TestExecute_NilRegistry_SkipsCheck(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:    5 * time.Second,
		MaxWorkers: 2,
		// SkillRegistry is nil — should skip skill validation
	})
	defer te.Close()

	// "shell" should still work because nil registry skips the check
	result, err := te.Execute(context.Background(), ToolCall{
		ID:   "tc-nil-reg",
		Name: "shell",
		Args: map[string]interface{}{"command": "echo works"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Output, "works")
}

// ---------------------------------------------------------------------------
// 5. TestExecuteWithTimeout_Success
// ---------------------------------------------------------------------------

func TestExecuteWithTimeout_Success(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       30 * time.Second,
		MaxWorkers:    2,
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	result, err := te.ExecuteWithTimeout(context.Background(), ToolCall{
		ID:   "tc-tw-1",
		Name: "shell",
		Args: map[string]interface{}{"command": "echo fast"},
	}, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Output, "fast")
}

// ---------------------------------------------------------------------------
// 6. TestExecuteWithTimeout_Exceeded
// ---------------------------------------------------------------------------

func TestExecuteWithTimeout_Exceeded(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       30 * time.Second,
		MaxWorkers:    2,
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	ctx := context.Background()
	_, err := te.ExecuteWithTimeout(ctx, ToolCall{
		ID:   "tc-tw-slow",
		Name: "shell",
		Args: map[string]interface{}{"command": "sleep 5"},
	}, 50*time.Millisecond)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// 7. TestToolPool_Execute
// ---------------------------------------------------------------------------

func TestToolPool_Execute(t *testing.T) {
	var called atomic.Int32
	pool := NewToolPool(ToolPoolConfig{
		MaxWorkers: 2,
		Executor: func(ctx context.Context, call ToolCall) (*ToolResult, error) {
			called.Add(1)
			return &ToolResult{CallID: call.ID, Name: call.Name, Output: "ok"}, nil
		},
	})
	defer pool.Close()

	result, err := pool.Execute(context.Background(), ToolCall{ID: "p1", Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Output)
	assert.Equal(t, int32(1), called.Load())
}

// ---------------------------------------------------------------------------
// 8. TestToolPool_ExecuteBatch
// ---------------------------------------------------------------------------

func TestToolPool_ExecuteBatch(t *testing.T) {
	pool := NewToolPool(ToolPoolConfig{
		MaxWorkers: 4,
		Executor: func(ctx context.Context, call ToolCall) (*ToolResult, error) {
			return &ToolResult{CallID: call.ID, Name: call.Name, Output: "batch-" + call.ID}, nil
		},
	})
	defer pool.Close()

	calls := []ToolCall{
		{ID: "b1", Name: "t1"},
		{ID: "b2", Name: "t2"},
		{ID: "b3", Name: "t3"},
	}
	results, err := pool.ExecuteBatch(context.Background(), calls)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "batch-b1", results[0].Output)
	assert.Equal(t, "batch-b2", results[1].Output)
	assert.Equal(t, "batch-b3", results[2].Output)
}

// ---------------------------------------------------------------------------
// 9. TestToolPool_ExecuteBatch_ErrorAggregation
// ---------------------------------------------------------------------------

func TestToolPool_ExecuteBatch_ErrorAggregation(t *testing.T) {
	pool := NewToolPool(ToolPoolConfig{
		MaxWorkers: 4,
		Executor: func(ctx context.Context, call ToolCall) (*ToolResult, error) {
			if call.ID == "fail" {
				return nil, context.DeadlineExceeded
			}
			return &ToolResult{CallID: call.ID, Output: "ok"}, nil
		},
	})
	defer pool.Close()

	results, err := pool.ExecuteBatch(context.Background(), []ToolCall{
		{ID: "ok1", Name: "t"},
		{ID: "fail", Name: "t"},
		{ID: "ok2", Name: "t"},
	})
	assert.Error(t, err, "batch should report error when any call fails")
	require.Len(t, results, 3)
	assert.Equal(t, "ok", results[0].Output)
	assert.Nil(t, results[1], "failed call should have nil result")
	assert.Equal(t, "ok", results[2].Output)
}

// ---------------------------------------------------------------------------
// 10. TestToolPool_Close
// ---------------------------------------------------------------------------

func TestToolPool_Close(t *testing.T) {
	pool := NewToolPool(ToolPoolConfig{
		MaxWorkers: 2,
		Executor: func(ctx context.Context, call ToolCall) (*ToolResult, error) {
			return &ToolResult{Output: "done"}, nil
		},
	})

	// Submit one task to ensure workers are alive
	_, err := pool.Execute(context.Background(), ToolCall{ID: "c1", Name: "close-test"})
	require.NoError(t, err)

	assert.NoError(t, pool.Close(), "Close should not error")
}

// ---------------------------------------------------------------------------
// 11. TestParseCommand
// ---------------------------------------------------------------------------

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple", `echo hello`, []string{"echo", "hello"}},
		{"double quotes", `echo "hello world"`, []string{"echo", "hello world"}},
		{"single quotes", `echo 'hello world'`, []string{"echo", "hello world"}},
		{"mixed quotes", `echo "it's" 'a test'`, []string{"echo", "it's", "a test"}},
		{"empty", ``, nil},
		{"extra spaces", `  echo   hello  `, []string{"echo", "hello"}},
		{"nested different quotes", `echo "say 'hi'"`, []string{"echo", "say 'hi'"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCommand(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// 12. TestIsValidJSON
// ---------------------------------------------------------------------------

func TestIsValidJSON(t *testing.T) {
	assert.True(t, isValidJSON(`{"key": "value"}`))
	assert.True(t, isValidJSON(`[1, 2, 3]`))
	assert.True(t, isValidJSON(`null`))
	assert.True(t, isValidJSON(`"hello"`))
	assert.False(t, isValidJSON(`not json`))
	assert.False(t, isValidJSON(`{broken`))
	assert.False(t, isValidJSON(``))
}

// ---------------------------------------------------------------------------
// 13. TestTruncateOutput
// ---------------------------------------------------------------------------

func TestTruncateOutput(t *testing.T) {
	assert.Equal(t, "short", truncateOutput("short", 100), "short string unchanged")
	assert.Equal(t, "short", truncateOutput("short", 5), "exact length unchanged")
	long := strings.Repeat("x", 200)
	result := truncateOutput(long, 100)
	assert.Equal(t, 100+len("\n... (truncated)"), len(result))
	assert.True(t, strings.HasSuffix(result, "\n... (truncated)"))
}

// ---------------------------------------------------------------------------
// 14. TestReadOutput
// ---------------------------------------------------------------------------

func TestReadOutput(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		out, err := readOutput(strings.NewReader("hello"), 100)
		assert.NoError(t, err)
		assert.Equal(t, "hello", out)
	})

	t.Run("over limit", func(t *testing.T) {
		big := strings.NewReader(strings.Repeat("a", 200))
		out, err := readOutput(big, 100)
		assert.NoError(t, err)
		assert.Len(t, out, 100+len("\n... (truncated)"))
		assert.Contains(t, out, "... (truncated)")
	})
}

// ---------------------------------------------------------------------------
// 15. TestToolPool_Execute_CancelledContext
// ---------------------------------------------------------------------------

func TestToolPool_Execute_CancelledContext(t *testing.T) {
	pool := NewToolPool(ToolPoolConfig{
		MaxWorkers: 1,
		Executor: func(ctx context.Context, call ToolCall) (*ToolResult, error) {
			return &ToolResult{Output: "done"}, nil
		},
	})
	defer pool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := pool.Execute(ctx, ToolCall{ID: "cancelled", Name: "test"})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// ---------------------------------------------------------------------------
// 16. TestExecute_MissingCommandArg
// ---------------------------------------------------------------------------

func TestExecute_MissingCommandArg(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       5 * time.Second,
		MaxWorkers:    2,
		SkillRegistry: shellRegistry(),
	})
	defer te.Close()

	_, err := te.Execute(context.Background(), ToolCall{
		ID:   "tc-missing-cmd",
		Name: "shell",
		Args: map[string]interface{}{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing or invalid")
}

// ---------------------------------------------------------------------------
// 17. TestExecute_UnsupportedDirectTool
// ---------------------------------------------------------------------------

func TestExecute_UnsupportedDirectTool(t *testing.T) {
	te := NewToolExecutor(ToolExecutorConfig{
		Timeout:       5 * time.Second,
		MaxWorkers:    2,
		SkillRegistry: nil, // nil → skip skill check
	})
	defer te.Close()

	// Name is not "shell" so executeDirect returns unsupported error
	_, err := te.Execute(context.Background(), ToolCall{
		ID:   "tc-unsupported",
		Name: "custom_tool",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported tool")
}
