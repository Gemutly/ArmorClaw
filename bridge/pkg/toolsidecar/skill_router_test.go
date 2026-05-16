package toolsidecar

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDockerClient struct {
	mu          sync.Mutex
	containers  map[string]bool
	nextID      int
	createErr   error
}

func newMockDockerClient() *mockDockerClient {
	return &mockDockerClient{
		containers: make(map[string]bool),
		nextID:     1,
	}
}

func (m *mockDockerClient) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, _ any, _ any, name string) (container.CreateResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return container.CreateResponse{}, m.createErr
	}
	id := fmt.Sprintf("container-%d", m.nextID)
	m.nextID++
	m.containers[id] = true
	return container.CreateResponse{ID: id}, nil
}

func (m *mockDockerClient) ContainerStart(_ context.Context, id string, _ container.StartOptions) error {
	return nil
}

func (m *mockDockerClient) ContainerStop(_ context.Context, id string, _ container.StopOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.containers, id)
	return nil
}

func (m *mockDockerClient) ContainerRemove(_ context.Context, id string, _ container.RemoveOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.containers, id)
	return nil
}

func (m *mockDockerClient) ContainerExecCreate(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
	return container.ExecCreateResponse{ID: "exec-1"}, nil
}

func (m *mockDockerClient) ContainerExecAttach(_ context.Context, _ string, _ container.ExecAttachOptions) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, nil
}

func (m *mockDockerClient) ContainerExecInspect(_ context.Context, _ string) (container.ExecInspect, error) {
	return container.ExecInspect{ExitCode: 0}, nil
}

func setupTestRouter(t *testing.T) (*SkillRouter, *mockDockerClient) {
	t.Helper()
	mock := newMockDockerClient()
	prov, err := NewProvisioner(Config{
		DockerClient: mock,
		DefaultImage: "armorclaw/toolsidecar:latest",
	})
	require.NoError(t, err)

	router, err := NewSkillRouter(SkillRouterConfig{
		Provisioner: prov,
		ImageMappings: []SkillImageMapping{
			{SkillName: "document_processing", Image: "armorclaw/sidecar-python:latest"},
			{SkillName: "browser_automation", Image: "armorclaw/jetski:latest"},
		},
		IdleTimeout:    1 * time.Hour,
		MaxPerWorkflow: 3,
	})
	require.NoError(t, err)
	return router, mock
}

func TestSkillRouting_ImageMapping(t *testing.T) {
	router, _ := setupTestRouter(t)

	img, ok := router.GetImageForSkill("document_processing")
	assert.True(t, ok)
	assert.Equal(t, "armorclaw/sidecar-python:latest", img)

	img, ok = router.GetImageForSkill("browser_automation")
	assert.True(t, ok)
	assert.Equal(t, "armorclaw/jetski:latest", img)

	_, ok = router.GetImageForSkill("unknown_skill")
	assert.False(t, ok)
}

func TestSkillRouting_SpawnWithSkill(t *testing.T) {
	router, _ := setupTestRouter(t)

	sidecar, err := router.Spawn(context.Background(), "document_processing", "sess-1", "wf-1")
	require.NoError(t, err)
	assert.Equal(t, "document_processing", sidecar.SkillName)
	assert.Equal(t, "running", sidecar.Status)
	assert.Equal(t, 1, router.ActiveCount())
	assert.Equal(t, 1, router.WorkflowCount("wf-1"))
}

func TestSkillRouting_SpawnDefaultImage(t *testing.T) {
	router, _ := setupTestRouter(t)

	sidecar, err := router.Spawn(context.Background(), "unknown_skill", "sess-2", "wf-2")
	require.NoError(t, err)
	assert.Equal(t, "unknown_skill", sidecar.SkillName)
	assert.Equal(t, 1, router.ActiveCount())
}

func TestSkillRouting_StopSidecar(t *testing.T) {
	router, _ := setupTestRouter(t)

	sidecar, err := router.Spawn(context.Background(), "document_processing", "sess-3", "wf-3")
	require.NoError(t, err)

	err = router.Stop(context.Background(), sidecar.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, router.ActiveCount())
	assert.Equal(t, 0, router.WorkflowCount("wf-3"))
}

func TestSkillRouting_ListSidecars(t *testing.T) {
	router, _ := setupTestRouter(t)

	_, err := router.Spawn(context.Background(), "document_processing", "sess-a", "wf-list")
	require.NoError(t, err)
	_, err = router.Spawn(context.Background(), "browser_automation", "sess-b", "wf-list")
	require.NoError(t, err)

	list := router.List()
	assert.Len(t, list, 2)
}

func TestSkillRouting_QuotaEnforcement(t *testing.T) {
	router, _ := setupTestRouter(t)

	for i := 0; i < 3; i++ {
		_, err := router.Spawn(context.Background(), "document_processing", fmt.Sprintf("sess-q-%d", i), "wf-quota")
		require.NoError(t, err)
	}

	_, err := router.Spawn(context.Background(), "document_processing", "sess-q-overflow", "wf-quota")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max concurrent sidecars")

	assert.Equal(t, 3, router.ActiveCount())
	assert.Equal(t, 3, router.WorkflowCount("wf-quota"))
}

func TestSkillRouting_DifferentWorkflowsIndependent(t *testing.T) {
	router, _ := setupTestRouter(t)

	_, err := router.Spawn(context.Background(), "document_processing", "sess-w1", "wf-alpha")
	require.NoError(t, err)
	_, err = router.Spawn(context.Background(), "document_processing", "sess-w2", "wf-beta")
	require.NoError(t, err)

	assert.Equal(t, 2, router.ActiveCount())
	assert.Equal(t, 1, router.WorkflowCount("wf-alpha"))
	assert.Equal(t, 1, router.WorkflowCount("wf-beta"))
}

func TestSkillRouting_StopNonExistentFails(t *testing.T) {
	router, _ := setupTestRouter(t)

	err := router.Stop(context.Background(), "nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSkillRouting_NilProvisionerFails(t *testing.T) {
	_, err := NewSkillRouter(SkillRouterConfig{
		Provisioner: nil,
	})
	assert.Error(t, err)
}

func TestSkillRouting_Defaults(t *testing.T) {
	mock := newMockDockerClient()
	prov, err := NewProvisioner(Config{
		DockerClient: mock,
		DefaultImage: "armorclaw/toolsidecar:latest",
	})
	require.NoError(t, err)

	router, err := NewSkillRouter(SkillRouterConfig{
		Provisioner: prov,
	})
	require.NoError(t, err)

	assert.Equal(t, 3, router.maxPerWorkflow)
	assert.Equal(t, 15*time.Minute, router.idleTimeout)
}

func TestSkillRouting_SpawnMissingSkill(t *testing.T) {
	router, _ := setupTestRouter(t)

	_, err := router.Spawn(context.Background(), "", "sess-1", "wf-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "skill_name is required")
}

func TestSkillRouting_SpawnMissingSession(t *testing.T) {
	router, _ := setupTestRouter(t)

	_, err := router.Spawn(context.Background(), "document_processing", "", "wf-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session_id is required")
}
