package studio

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func setupFactoryWithDef(t *testing.T, defID string) (*AgentFactory, *mockDockerClient) {
	t.Helper()

	store, err := NewStore(StoreConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	def := &AgentDefinition{
		ID:           defID,
		Name:         "Config Test Agent",
		Skills:       []string{"browser_navigate"},
		ResourceTier: "medium",
		CreatedBy:    "@test:example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IsActive:     true,
	}
	if err := store.CreateDefinition(def); err != nil {
		t.Fatalf("failed to create definition: %v", err)
	}

	mockDocker := &mockDockerClient{}
	factory := NewAgentFactory(FactoryConfig{
		DockerClient: mockDocker,
		Store:        store,
		StateDir:     os.TempDir(),
	})

	return factory, mockDocker
}

func findEnv(t *testing.T, env []string, prefix string) (string, bool) {
	t.Helper()
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e, true
		}
	}
	return "", false
}

func TestSpawnConfigPassthrough(t *testing.T) {
	factory, mockDocker := setupFactoryWithDef(t, "cfg-passthrough")

	_, err := factory.Spawn(context.Background(), &SpawnRequest{
		DefinitionID: "cfg-passthrough",
		UserID:       "@test:example.com",
		Config:       json.RawMessage(`{"model":"gpt-4","temperature":0.7}`),
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if len(mockDocker.createdContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(mockDocker.createdContainers))
	}

	val, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "STEP_CONFIG=")
	if !found {
		t.Fatal("STEP_CONFIG env var not found in container config")
	}

	expected := `STEP_CONFIG={"model":"gpt-4","temperature":0.7}`
	if val != expected {
		t.Errorf("expected %q, got %q", expected, val)
	}
}

func TestSpawnConfigNil(t *testing.T) {
	factory, mockDocker := setupFactoryWithDef(t, "cfg-nil")

	_, err := factory.Spawn(context.Background(), &SpawnRequest{
		DefinitionID: "cfg-nil",
		UserID:       "@test:example.com",
		Config:       nil,
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if len(mockDocker.createdContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(mockDocker.createdContainers))
	}

	_, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "STEP_CONFIG=")
	if found {
		t.Error("STEP_CONFIG should NOT be present when Config is nil")
	}
}

func TestSpawnConfigEmpty(t *testing.T) {
	factory, mockDocker := setupFactoryWithDef(t, "cfg-empty")

	_, err := factory.Spawn(context.Background(), &SpawnRequest{
		DefinitionID: "cfg-empty",
		UserID:       "@test:example.com",
		Config:       json.RawMessage{},
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if len(mockDocker.createdContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(mockDocker.createdContainers))
	}

	_, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "STEP_CONFIG=")
	if found {
		t.Error("STEP_CONFIG should NOT be present when Config is empty")
	}
}

func TestSpawnConfigRawJSON(t *testing.T) {
	factory, mockDocker := setupFactoryWithDef(t, "cfg-raw")

	_, err := factory.Spawn(context.Background(), &SpawnRequest{
		DefinitionID: "cfg-raw",
		UserID:       "@test:example.com",
		Config:       json.RawMessage("not-json"),
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if len(mockDocker.createdContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(mockDocker.createdContainers))
	}

	val, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "STEP_CONFIG=")
	if !found {
		t.Fatal("STEP_CONFIG env var not found in container config")
	}

	expected := "STEP_CONFIG=not-json"
	if val != expected {
		t.Errorf("expected %q, got %q", expected, val)
	}
}

func TestAgentModeInjectsFileWriterEnv(t *testing.T) {
	factory, mockDocker := setupFactoryWithDef(t, "agent-file-writer")

	_, err := factory.Spawn(context.Background(), &SpawnRequest{
		DefinitionID: "agent-file-writer",
		UserID:       "@test:example.com",
		Config:        nil,
	})
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	if len(mockDocker.createdContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(mockDocker.createdContainers))
	}

	val, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "AGENT_FILE_WRITER_ENABLED=")
	if !found {
		t.Fatal("AGENT_FILE_WRITER_ENABLED env var not found in container config (agent mode)")
	}
	if val != "AGENT_FILE_WRITER_ENABLED=1" {
		t.Errorf("expected AGENT_FILE_WRITER_ENABLED=1, got %q", val)
	}

	val, found = findEnv(t, mockDocker.createdContainers[0].config.Env, "AGENT_STATUS_DIR=")
	if !found {
		t.Fatal("AGENT_STATUS_DIR env var not found in container config (agent mode)")
	}
	if val != "AGENT_STATUS_DIR=/home/claw/.openclaw" {
		t.Errorf("expected AGENT_STATUS_DIR=/home/claw/.openclaw, got %q", val)
	}

	if _, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "STEP_CONFIG="); found {
		t.Error("STEP_CONFIG should NOT be present in agent mode")
	}
}

func TestStepModeDoesNotInjectFileWriterEnv(t *testing.T) {
	factory, mockDocker := setupFactoryWithDef(t, "step-no-writer")

	_, err := factory.Spawn(context.Background(), &SpawnRequest{
		DefinitionID: "step-no-writer",
		UserID:       "@test:example.com",
		Config:        []byte(`{"model":"gpt-4","temperature":0.7}`),
	})
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}

	if len(mockDocker.createdContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(mockDocker.createdContainers))
	}

	if _, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "AGENT_FILE_WRITER_ENABLED="); found {
		t.Error("AGENT_FILE_WRITER_ENABLED should NOT be present in step mode")
	}

	if _, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "AGENT_STATUS_DIR="); found {
		t.Error("AGENT_STATUS_DIR should NOT be present in step mode")
	}

	val, found := findEnv(t, mockDocker.createdContainers[0].config.Env, "STEP_CONFIG=")
	if !found {
		t.Fatal("STEP_CONFIG env var not found in container config (step mode)")
	}
	if val != `STEP_CONFIG={"model":"gpt-4","temperature":0.7}` {
		t.Errorf("STEP_CONFIG value mismatch, got %q", val)
	}
}
