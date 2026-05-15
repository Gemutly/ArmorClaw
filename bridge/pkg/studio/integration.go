package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

//=============================================================================
// Docker Client Adapter
//=============================================================================

// DockerClientAdapter wraps the bridge's Docker client to implement
// the studio's DockerClient interface
type DockerClientAdapter struct {
	createFunc  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (string, error)
	startFunc   func(ctx context.Context, containerID string) error
	stopFunc    func(ctx context.Context, containerID string, timeout time.Duration) error
	removeFunc  func(ctx context.Context, containerID string, force bool) error
	inspectFunc func(ctx context.Context, containerID string) (*ContainerInfo, error)
	listFunc    func(ctx context.Context, all bool) ([]types.Container, error)
	killFunc    func(ctx context.Context, containerID string, signal string) error
}

// ContainerInfo contains container state information
type ContainerInfo struct {
	ID       string
	Running  bool
	ExitCode int
}

// NewDockerClientAdapter creates an adapter from function implementations
func NewDockerClientAdapter(
	create func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (string, error),
	start func(ctx context.Context, containerID string) error,
	stop func(ctx context.Context, containerID string, timeout time.Duration) error,
	remove func(ctx context.Context, containerID string, force bool) error,
	inspect func(ctx context.Context, containerID string) (*ContainerInfo, error),
	list func(ctx context.Context, all bool) ([]types.Container, error),
) *DockerClientAdapter {
	return &DockerClientAdapter{
		createFunc:  create,
		startFunc:   start,
		stopFunc:    stop,
		removeFunc:  remove,
		inspectFunc: inspect,
		listFunc:    list,
	}
}

// SetKillFunc sets the kill function for SIGKILL support (optional, called after construction if available)
func (a *DockerClientAdapter) SetKillFunc(kill func(ctx context.Context, containerID string, signal string) error) {
	a.killFunc = kill
}

func (a *DockerClientAdapter) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig any, platform any, name string) (container.CreateResponse, error) {
	id, err := a.createFunc(ctx, config, hostConfig, name)
	if err != nil {
		return container.CreateResponse{}, err
	}
	return container.CreateResponse{ID: id}, nil
}

func (a *DockerClientAdapter) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return a.startFunc(ctx, containerID)
}

func (a *DockerClientAdapter) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	timeout := 30 * time.Second
	if options.Timeout != nil {
		timeout = time.Duration(*options.Timeout) * time.Second
	}
	return a.stopFunc(ctx, containerID, timeout)
}

func (a *DockerClientAdapter) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return a.removeFunc(ctx, containerID, options.Force)
}

func (a *DockerClientAdapter) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	info, err := a.inspectFunc(ctx, containerID)
	if err != nil {
		return types.ContainerJSON{}, err
	}
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: info.ID,
			State: &types.ContainerState{
				Running:  info.Running,
				ExitCode: info.ExitCode,
			},
		},
	}, nil
}

func (a *DockerClientAdapter) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return a.listFunc(ctx, options.All)
}

func (a *DockerClientAdapter) ContainerKill(ctx context.Context, containerID string, signal string) error {
	if a.killFunc != nil {
		return a.killFunc(ctx, containerID, signal)
	}
	return fmt.Errorf("kill function not configured on DockerClientAdapter")
}

//=============================================================================
// Studio Integration
//=============================================================================

// StudioIntegration provides a complete Agent Studio setup
// This is the main entry point for integrating the studio into ArmorClaw
type StudioIntegration struct {
	store    Store
	rpc      *RPCHandler
	commands *CommandHandler
	factory  *AgentFactory
}

// IntegrationConfig contains all dependencies for the studio
type IntegrationConfig struct {
	// Database path for studio persistence (empty for in-memory)
	DataPath string

	// Docker client for container spawning (uses adapter pattern)
	DockerClient DockerClient

	// Matrix adapter for command handling (optional)
	MatrixAdapter MatrixAdapter

	// Command prefix for Matrix commands (default: "!")
	CommandPrefix string

	// Wizard timeout in minutes (default: 5)
	WizardTimeout int
}

// NewIntegration creates a complete studio integration
func NewIntegration(cfg IntegrationConfig) (*StudioIntegration, error) {
	// 1. Create store
	storePath := cfg.DataPath
	if storePath == "" {
		storePath = ":memory:"
	}

	store, err := NewStore(StoreConfig{Path: storePath})
	if err != nil {
		return nil, fmt.Errorf("failed to create studio store: %w", err)
	}

	// 2. Create factory FIRST (before RPC handler, so it can be injected)
	var factory *AgentFactory
	if cfg.DockerClient != nil {
		factory = NewAgentFactory(FactoryConfig{
			DockerClient: cfg.DockerClient,
			Store:        store,
		})
	}

	// 3. Create RPC handler with factory
	rpcHandler := NewRPCHandler(RPCHandlerConfig{
		Store:   store,
		Factory: factory,
	})

	// 4. Create command handler (if Matrix adapter provided)
	var commandHandler *CommandHandler
	if cfg.MatrixAdapter != nil {
		wizardTimeout := cfg.WizardTimeout
		if wizardTimeout == 0 {
			wizardTimeout = 5
		}

		prefix := cfg.CommandPrefix
		if prefix == "" {
			prefix = "!"
		}

		commandHandler = NewCommandHandler(CommandHandlerConfig{
			Store:         store,
			Factory:       factory,
			Matrix:        cfg.MatrixAdapter,
			CommandPrefix: prefix,
			WizardTimeout: 0,
		})
	}

	return &StudioIntegration{
		store:    store,
		rpc:      rpcHandler,
		commands: commandHandler,
		factory:  factory,
	}, nil
}

// GetRPCHandler returns the RPC handler for registering with the RPC server
func (s *StudioIntegration) GetRPCHandler() *RPCHandler {
	return s.rpc
}

// GetCommandHandler returns the Matrix command handler
func (s *StudioIntegration) GetCommandHandler() *CommandHandler {
	return s.commands
}

// GetFactory returns the agent factory for container spawning
func (s *StudioIntegration) GetFactory() *AgentFactory {
	return s.factory
}

// GetStore returns the underlying store
func (s *StudioIntegration) GetStore() Store {
	return s.store
}

// HandleRPCMethod routes studio.* methods to the appropriate handler
// This can be called from the main RPC server's switch statement
func (s *StudioIntegration) HandleRPCMethod(method string, params json.RawMessage) *RPCResponse {
	req := &RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return s.rpc.Handle(req)
}

// HandleMatrixMessage processes Matrix messages for studio commands
// Returns true if the message was handled as a studio command
func (s *StudioIntegration) HandleMatrixMessage(ctx context.Context, roomID, userID, eventID, text string) bool {
	if s.commands == nil {
		return false
	}
	handled, err := s.commands.HandleMessage(ctx, roomID, userID, eventID, text)
	if err != nil {
		log.Printf("Studio command error: %v", err)
	}
	return handled
}

// Close shuts down the studio integration
func (s *StudioIntegration) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

//=============================================================================
// RPC Method Registration Helper
//=============================================================================

// StudioMethods returns a list of all studio.* RPC method names
// This can be used for documentation or validation
var StudioMethods = []string{
	"studio.list_skills",
	"studio.get_skill",
	"studio.register_skill",
	"studio.list_pii",
	"studio.get_pii",
	"studio.register_pii",
	"studio.list_profiles",
	"studio.create_agent",
	"studio.update_agent",
	"studio.delete_agent",
	"studio.list_agents",
	"studio.get_agent",
	"studio.spawn_agent",
	"studio.list_instances",
	"studio.stop_instance",
	"studio.get_instance",
	"studio.stats",
	// MCP Registry
	"studio.list_mcps",
	"studio.get_mcp",
	"studio.get_mcp_warning",
	// MCP Approval Workflow
	"studio.request_mcp_approval",
	"studio.list_pending_approvals",
	"studio.list_my_approvals",
	"studio.approve_mcp_request",
	"studio.reject_mcp_request",
}

// IsStudioMethod checks if a method name is a studio method
func IsStudioMethod(method string) bool {
	return len(method) > 7 && method[:7] == "studio."
}

//=============================================================================
// Example Integration Code
//=============================================================================

/*
Example integration with the main RPC server (in pkg/rpc/server.go):

1. Add studio to Server struct:

	type Server struct {
		// ... existing fields ...
		studio *studio.StudioIntegration
	}

2. Add studio to Config struct:

	type Config struct {
		// ... existing fields ...

		// Studio configuration (Agent Factory)
		StudioDataPath string
	}

3. Initialize studio in New():

	var studioIntegration *studio.StudioIntegration
	if cfg.StudioDataPath != "" || true { // Always enable for now
		studioIntegration, err = studio.NewIntegration(studio.IntegrationConfig{
			DataPath:      cfg.StudioDataPath,
			DockerClient:  dockerClient, // from earlier in New()
			MatrixAdapter: matrixAdapter, // if configured
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize studio: %v", err)
		}
	}
	server.studio = studioIntegration

4. Add studio methods to handleRequest switch:

	// Studio methods (Agent Factory)
	case "studio.list_skills", "studio.get_skill", "studio.register_skill",
	     "studio.list_pii", "studio.get_pii", "studio.register_pii",
	     "studio.list_profiles", "studio.create_agent", "studio.update_agent",
	     "studio.delete_agent", "studio.list_agents", "studio.get_agent",
	     "studio.spawn_agent", "studio.list_instances", "studio.stop_instance",
	     "studio.get_stats":
		if s.studio != nil {
			return s.studio.HandleRPCMethod(req.Method, req.Params)
		}
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &ErrorObj{
				Code:    MethodNotFound,
				Message: "studio not initialized",
			},
		}

5. For Matrix command handling (in the Matrix adapter):

	func (a *MatrixAdapter) HandleMessage(ctx context.Context, roomID, userID, eventID, body string) {
		// Check for studio commands first
		if a.studio != nil {
			if a.studio.HandleMatrixMessage(ctx, roomID, userID, eventID, body) {
				return // Studio handled it
			}
		}
		// ... existing message handling ...
	}
*/
