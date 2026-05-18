package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armorclaw/bridge/internal/adapter"
	"github.com/armorclaw/bridge/internal/ai"
	"github.com/armorclaw/bridge/internal/events"
	"github.com/armorclaw/bridge/internal/skills"
	"github.com/armorclaw/bridge/pkg/appservice"
	"github.com/armorclaw/bridge/pkg/audit"
	"github.com/armorclaw/bridge/pkg/browser"
	"github.com/armorclaw/bridge/pkg/crypto"
	"github.com/armorclaw/bridge/pkg/docker"
	"github.com/armorclaw/bridge/pkg/eventbus"
	"github.com/armorclaw/bridge/pkg/eventlog"
	"github.com/armorclaw/bridge/pkg/interfaces"
	"github.com/armorclaw/bridge/pkg/invite"
	"github.com/armorclaw/bridge/pkg/keystore"
	"github.com/armorclaw/bridge/pkg/mcp"
	"github.com/armorclaw/bridge/pkg/provisioning"
	"github.com/armorclaw/bridge/pkg/secretary"
	"github.com/armorclaw/bridge/pkg/sidecar"
	"github.com/armorclaw/bridge/pkg/studio"
	"github.com/armorclaw/bridge/pkg/translator"
	"github.com/armorclaw/bridge/pkg/trust"
	"github.com/armorclaw/bridge/pkg/voice"
)

// ReplayFeatureFlags holds feature toggles needed by browser replay RPC handlers.
type ReplayFeatureFlags struct {
	MultiTabReplay bool
}

const (
	JSONRPCVersion      = "2.0"
	BridgeVersion       = "4.6.0"
	ParseError          = -32700
	InvalidRequest      = -32600
	MethodNotFound      = -32601
	InvalidParams       = -32602
	InternalError       = -32603
	NotFoundError       = -32000
	TooManyRequests     = -32001
	RequestCancelled    = -32002
	KeystoreRateLimited = -32006
)

type BridgeManager interface {
	Start() error
	Stop() error
	RegisterAdapter(platform appservice.Platform, adapter interface{}) error
	BridgeChannel(matrixRoomID string, platform appservice.Platform, channelID string) error
	UnbridgeChannel(platform appservice.Platform, channelID string) error
	GetBridgedChannels() []*appservice.BridgedChannel
	GetStats() map[string]interface{}
}

type StudioService interface {
	HandleRPCMethod(method string, params json.RawMessage) *studio.RPCResponse
}

type ProvisioningManager interface {
	GetUserRole(userID string) provisioning.AdminRole
	StartSetupToken(ctx context.Context) (qrDeepLink string, expiresAt string, err error)
}

type AppService interface {
	GetStats() map[string]interface{}
}

type SkillManager interface {
	ExecuteSkill(ctx context.Context, skillName string, params map[string]interface{}) (*skills.SkillResult, error)
	ListEnabled() []*skills.Skill
	GetSkill(skillName string) (*skills.Skill, bool)
	AllowSkill(skillName string) error
	BlockSkill(skillName string) error
	AllowIP(ip string) error
	AllowCIDR(cidr string) error
	GetAllowlist() ([]string, []string)
	GenerateSchema(skill *skills.Skill) interface{}
}

type TLSInfoProvider interface {
	GetTLSInfo() interface{}
}

type MethodRateLimiter interface {
	CheckRateLimit(method string) error
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`

	// AuthToken carries the bearer token extracted from HTTP Authorization
	// header (TCP connections via POST /api). Not serialized in JSON-RPC.
	AuthToken string `json:"-"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *ErrorObj   `json:"error,omitempty"`
}

type ErrorObj struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Keystore interface{}
type MatrixAdapter interface {
	SendMessage(roomID, message, msgType string) (string, error)
	SendEvent(roomID, eventType string, content []byte) error
	Login(username, password string) error
	JoinRoom(ctx context.Context, roomIDOrAlias string, viaServers []string, reason string) (string, error)
	GetUserID() string
	IsLoggedIn() bool
	GetHomeserver() string
	GetStatus() string
}

type MatrixHealthResult struct {
	Enabled    bool   `json:"enabled"`
	Connected  bool   `json:"connected"`
	LoggedIn   bool   `json:"logged_in"`
	Homeserver string `json:"homeserver"`
	UserID     string `json:"user_id,omitempty"`
	LastSync   string `json:"last_sync,omitempty"`
	Error      string `json:"error,omitempty"`
}

type HealthCheckResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}

type HandlerFunc func(ctx context.Context, req *Request) (interface{}, *ErrorObj)

type FeatureFlags struct {
	ZeroTrustKeystore bool
	VoicePipeline     string
	MultiTabReplay    bool
	E2EEBackup        bool
}

type Server struct {
	keystore          Keystore
	matrix            MatrixAdapter
	aiService         *ai.AIService
	aiSemaphore       chan struct{}
	aiMaxConcurrent   int
	bridgeMgr         BridgeManager
	browserJobs       *BrowserJobManager
	browserBroker     browser.BrowserBroker
	studio            StudioService
	appService        AppService
	provisioningMgr   ProvisioningManager
	skillMgr          SkillManager
	skillGate         interfaces.SkillGate
	mcpRouter         *mcp.MCPRouter
	translator        *translator.RPCToMCPTranslator
	eventBus          *eventbus.EventBus
	handlers          map[string]HandlerFunc
	hardeningStore    trust.Store
	deviceStore       *trust.DeviceStore
	inviteStore       *invite.InviteStore
	secretaryHandler  secretaryRPCHandler
	artifactHandler   *artifactRPCHandlerAdapter
	heartbeats        sync.Map
	metrics           *Metrics
	listener          net.Listener
	shutdownCh        chan struct{}
	rpcTransport      string
	listenAddr        string
	dockerClient      *docker.Client
	guard             *trust.TrustedProxyGuard
	auditLog          *audit.AuditLog
	governanceRoomID  string
	tlsInfoProvider   TLSInfoProvider
	piiRequestManager *keystore.PIIRequestManager
	e2eeEnabled       atomic.Bool
	sealedKS          *keystore.SealedKeystore
	keystoreLimiter   *keystore.RateLimiter
	zeroTrustKS       bool
	backupMgr         *crypto.BackupManager
	e2eeBackupEnabled bool
	voiceMgr          *voice.Manager
	voicePipeline     string
	replayFlags       ReplayFeatureFlags
	navChartStore     *browser.MultiTabStore
	methodRateLimiter MethodRateLimiter
	sidecarOffice     *sidecar.Client
	sidecarRust       *sidecar.Client
	sidecarJava       *sidecar.Client
	extractionJobs    map[string]*ExtractionJob
	outboxStore       OutboxStoreReader
	safety            *SafetyMiddleware
}

type Config struct {
	SocketPath        string
	RPCTransport      string
	ListenAddr        string
	Keystore          Keystore
	Matrix            MatrixAdapter
	AIService         *ai.AIService
	AIMaxConcurrent   int
	BridgeManager     BridgeManager
	BrowserJobs       *BrowserJobManager
	Studio            StudioService
	AppService        AppService
	ProvisioningMgr   ProvisioningManager
	SkillManager      SkillManager
	SkillGate         interfaces.SkillGate
	EventBus          *eventbus.EventBus
	HardeningStore    trust.Store
	DeviceStore       *trust.DeviceStore
	InviteStore       *invite.InviteStore
	Metrics           *Metrics
	DockerClient      *docker.Client
	Guard             *trust.TrustedProxyGuard
	AuditLog          *audit.AuditLog
	MCPRouter         *mcp.MCPRouter
	GovernanceRoomID  string
	Translator        *translator.RPCToMCPTranslator
	SecretaryHandler  secretaryRPCHandler
	ArtifactHandler   *artifactRPCHandlerAdapter
	SealedKS          *keystore.SealedKeystore
	KeystoreLimiter   *keystore.RateLimiter
	ZeroTrustKS       bool
	BackupMgr         *crypto.BackupManager
	E2EEBackupEnabled bool
	EnableE2EE        bool
	VoiceMgr          *voice.Manager
	VoicePipeline     string
	ReplayFlags       ReplayFeatureFlags
	NavChartStore     *browser.MultiTabStore
	MethodRateLimiter MethodRateLimiter
	OutboxStore       OutboxStoreReader
	AdminToken        string
}

func New(cfg Config) (*Server, error) {
	if cfg.AIMaxConcurrent <= 0 {
		cfg.AIMaxConcurrent = 4
	}

	s := &Server{
		keystore:          cfg.Keystore,
		matrix:            cfg.Matrix,
		aiService:         cfg.AIService,
		aiMaxConcurrent:   cfg.AIMaxConcurrent,
		aiSemaphore:       make(chan struct{}, cfg.AIMaxConcurrent),
		bridgeMgr:         cfg.BridgeManager,
		browserJobs:       cfg.BrowserJobs,
		studio:            cfg.Studio,
		appService:        cfg.AppService,
		provisioningMgr:   cfg.ProvisioningMgr,
		skillMgr:          cfg.SkillManager,
		skillGate:         cfg.SkillGate,
		eventBus:          cfg.EventBus,
		handlers:          make(map[string]HandlerFunc, 32),
		hardeningStore:    cfg.HardeningStore,
		deviceStore:       cfg.DeviceStore,
		inviteStore:       cfg.InviteStore,
		metrics:           cfg.Metrics,
		shutdownCh:        make(chan struct{}),
		rpcTransport:      cfg.RPCTransport,
		listenAddr:        cfg.ListenAddr,
		dockerClient:      cfg.DockerClient,
		guard:             cfg.Guard,
		auditLog:          cfg.AuditLog,
		mcpRouter:         cfg.MCPRouter,
		translator:        cfg.Translator,
		secretaryHandler:  cfg.SecretaryHandler,
		artifactHandler:   cfg.ArtifactHandler,
		governanceRoomID:  cfg.GovernanceRoomID,
		sealedKS:          cfg.SealedKS,
		keystoreLimiter:   cfg.KeystoreLimiter,
		zeroTrustKS:       cfg.ZeroTrustKS,
		backupMgr:         cfg.BackupMgr,
		e2eeBackupEnabled: cfg.E2EEBackupEnabled,
		voiceMgr:          cfg.VoiceMgr,
		voicePipeline:     cfg.VoicePipeline,
		replayFlags:       cfg.ReplayFlags,
		navChartStore:     cfg.NavChartStore,
		methodRateLimiter: cfg.MethodRateLimiter,
		outboxStore:       cfg.OutboxStore,
	}
	s.e2eeEnabled.Store(cfg.EnableE2EE)

	s.piiRequestManager = keystore.NewPIIRequestManager(keystore.PIIRequestManagerConfig{
		DefaultTTL: 5 * time.Minute,
	})

	s.safety = NewBEATOSafetyMiddleware(cfg.AdminToken)

	s.registerHandlers()
	return s, nil
}

// SetBrowserBroker injects a BrowserBroker implementation. When set, all
// browser.* RPC methods route through the broker instead of the legacy
// BrowserSkill stub. Pass nil to restore fallback behaviour.
func (s *Server) SetBrowserBroker(b browser.BrowserBroker) {
	s.browserBroker = b
}

func (s *Server) SetTLSInfoProvider(p TLSInfoProvider) {
	s.tlsInfoProvider = p
}

func (s *Server) Handle(ctx context.Context, req *Request) (resp *Response) {
	defer func() {
		if r := recover(); r != nil {
			var id interface{}
			var method string
			if req != nil {
				id = req.ID
				method = req.Method
			}
			slog.Error(
				"rpc_panic",
				"method", method,
				"id", id,
				"recover", r,
			)
			resp = errorResponse(id, InternalError, "internal server error")
		}
	}()

	if req == nil {
		return errorResponse(nil, InvalidRequest, "request is nil")
	}

	if s.metrics != nil {
		s.metrics.IncrementCounter("armorclaw_rpc_requests_total", req.Method)
	}

	isNotification := req.ID == nil

	if req.JSONRPC != JSONRPCVersion {
		if isNotification {
			return nil
		}
		return errorResponse(req.ID, InvalidRequest, "invalid jsonrpc version")
	}

	if req.Method == "" {
		slog.Warn("rpc_invalid_method")
		if isNotification {
			return nil
		}
		return errorResponse(req.ID, InvalidRequest, "method is required")
	}

	handler, ok := s.handlers[req.Method]
	if !ok {
		if isNotification {
			return nil
		}
		return errorResponse(req.ID, MethodNotFound, "method not found")
	}

	if s.methodRateLimiter != nil {
		if err := s.methodRateLimiter.CheckRateLimit(req.Method); err != nil {
			if isNotification {
				return nil
			}
			return &Response{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Error:   &ErrorObj{Code: KeystoreRateLimited, Message: err.Error()},
			}
		}
	}

	result, rpcErr := handler(ctx, req)

	if isNotification {
		return nil
	}

	if rpcErr != nil {
		return &Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
}

func errorResponse(id interface{}, code int, msg string) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error:   &ErrorObj{Code: code, Message: msg},
	}
}

// isInterfaceNil checks if an interface is truly nil.
// In Go, an interface with a nil concrete value is NOT nil.
// This helper handles that case by using reflection.
func isInterfaceNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch v := reflect.ValueOf(i); v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return v.IsNil()
	}
	return false
}

func (s *Server) handleHealthCheck(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	components := make(map[string]string)
	status := "unhealthy"

	components["bridge"] = "ok"

	if !isInterfaceNil(s.matrix) {
		if s.matrix.IsLoggedIn() {
			components["matrix"] = "connected"
		} else {
			components["matrix"] = "disconnected"
		}
		components["matrix_status"] = s.matrix.GetStatus()
	} else {
		components["matrix"] = "disconnected"
	}

	if !isInterfaceNil(s.keystore) {
		ks, ok := s.keystore.(*keystore.Keystore)
		if ok && ks != nil {
			if err := ks.Open(); err != nil {
				components["keystore"] = "error"
			} else {
				ks.Close()
				components["keystore"] = "initialized"
			}
		} else {
			components["keystore"] = "initialized"
		}
	} else {
		components["keystore"] = "error"
	}

	if components["bridge"] == "ok" && components["matrix"] == "connected" && components["keystore"] == "initialized" {
		status = "healthy"
	} else if components["bridge"] == "ok" {
		status = "degraded"
	}

	components["sidecar.extraction_mode"] = string(sidecar.GetExtractionMode())

	if s.auditLog != nil {
		_ = s.auditLog.LogEvent(audit.EventSidecarHealthCheck, "", "", "", map[string]interface{}{
			"status":     status,
			"components": components,
		})
	}

	return HealthCheckResponse{
		Status:     status,
		Components: components,
	}, nil
}

// Matrix Handlers

func (s *Server) handleMatrixStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if isInterfaceNil(s.matrix) {
		return MatrixHealthResult{
			Enabled:   false,
			Connected: false,
			Error:     "matrix adapter not configured",
		}, nil
	}

	result := MatrixHealthResult{
		Enabled:    true,
		Connected:  true,
		LoggedIn:   s.matrix.IsLoggedIn(),
		Homeserver: s.matrix.GetHomeserver(),
		UserID:     s.matrix.GetUserID(),
	}

	if !s.matrix.IsLoggedIn() {
		result.Error = "not logged in"
	}

	return result, nil
}

func (s *Server) handleMatrixLogin(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Errorf("invalid parameters: %w", err).Error(),
		}
	}

	matrix, ok := s.matrix.(*adapter.MatrixAdapter)
	if !ok || matrix == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "matrix adapter not configured",
		}
	}

	if err := matrix.Login(params.Username, params.Password); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Errorf("login failed: %w", err).Error(),
		}
	}

	// Start sync loop after successful login
	matrix.StartSync()

	return map[string]interface{}{
		"success": true,
		"user_id": matrix.GetUserID(),
	}, nil
}

func (s *Server) handleMatrixSend(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		RoomID  string `json:"room_id"`
		Message string `json:"message"`
		MsgType string `json:"msgtype"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Errorf("invalid parameters: %w", err).Error(),
		}
	}

	matrix, ok := s.matrix.(*adapter.MatrixAdapter)
	if !ok || matrix == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "matrix adapter not configured",
		}
	}

	eventID, err := matrix.SendMessage(params.RoomID, params.Message, params.MsgType)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Errorf("send failed: %w", err).Error(),
		}
	}

	if s.metrics != nil {
		s.metrics.IncrementCounter("armorclaw_matrix_messages_total", "sent")
	}

	return map[string]interface{}{
		"event_id": eventID,
		"room_id":  params.RoomID,
	}, nil
}

func (s *Server) handleMatrixReceive(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Cursor    string `json:"cursor"`
		TimeoutMs int    `json:"timeout_ms"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Errorf("invalid parameters: %w", err).Error(),
		}
	}

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 30000
	}

	matrix, ok := s.matrix.(*adapter.MatrixAdapter)
	if !ok || matrix == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "matrix adapter not configured",
		}
	}

	// If event bus is configured, use it for streaming
	if matrix.GetEventBus() != nil {
		return s.handleMatrixReceiveWithEventBus(ctx, req, &params, matrix)
	}

	// Fallback to polling old event queue
	return s.handleMatrixReceivePolling(ctx, req, &params, matrix)
}

// handleMatrixJoinRoom joins an existing Matrix room
func (s *Server) handleMatrixJoinRoom(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		RoomID     string   `json:"room_id"`
		ViaServers []string `json:"via_servers"`
		Reason     string   `json:"reason"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "failed to parse params",
		}
	}

	if params.RoomID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "room_id is required",
		}
	}

	matrix := s.GetMatrixAdapter()
	if matrix == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "matrix adapter not configured",
		}
	}

	joinedRoom, err := matrix.JoinRoom(ctx, params.RoomID, params.ViaServers, params.Reason)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: err.Error(),
		}
	}

	return map[string]string{"room_id": joinedRoom}, nil
}

func (s *Server) handleMatrixReceiveWithEventBus(ctx context.Context, req *Request, params *struct {
	Cursor    string `json:"cursor"`
	TimeoutMs int    `json:"timeout_ms"`
}, matrix *adapter.MatrixAdapter) (interface{}, *ErrorObj) {
	cursor, err := strconv.ParseUint(params.Cursor, 10, 64)
	if err != nil {
		cursor = 0
	}

	eventBus := matrix.GetEventBus()
	if eventBus == nil {
		return map[string]interface{}{
			"events":       []events.MatrixEvent{},
			"cursor":       strconv.FormatUint(cursor, 10),
			"count":        0,
			"cursor_reset": false,
		}, nil
	}

	timeout := time.Duration(params.TimeoutMs) * time.Millisecond
	recvCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	evs, next, reset := eventBus.WaitForEvents(recvCtx, cursor)

	if recvCtx.Err() != nil && len(evs) == 0 && !reset {
		return map[string]interface{}{
			"events":       []events.MatrixEvent{},
			"cursor":       strconv.FormatUint(cursor, 10),
			"count":        0,
			"cursor_reset": false,
		}, nil
	}

	return map[string]interface{}{
		"events":       evs,
		"cursor":       strconv.FormatUint(next, 10),
		"count":        len(evs),
		"cursor_reset": reset,
	}, nil
}

func (s *Server) handleMatrixReceivePolling(ctx context.Context, req *Request, params *struct {
	Cursor    string `json:"cursor"`
	TimeoutMs int    `json:"timeout_ms"`
}, matrix *adapter.MatrixAdapter) (interface{}, *ErrorObj) {
	timeout := time.Duration(params.TimeoutMs) * time.Millisecond
	select {
	case <-ctx.Done():
		return nil, &ErrorObj{
			Code:    RequestCancelled,
			Message: ctx.Err().Error(),
		}
	case <-time.After(timeout):
		// Poll the event queue (backward compatibility)
		events := make([]*adapter.MatrixEvent, 0, 10)
		timeoutChan := time.After(timeout)

		// Try to get events
		select {
		case e, ok := <-matrix.ReceiveEvents():
			if ok {
				events = append(events, e)
			}
		case <-timeoutChan:
			// Timed out
		}

		return map[string]interface{}{
			"events": events,
			"count":  len(events),
		}, nil
	}
}

func (s *Server) handleEventsReplay(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.eventBus == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "event bus not initialized"}
	}

	log := s.eventBus.GetLog()
	if log == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "durable log not enabled"}
	}

	var params struct {
		Offset uint64 `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	records, err := log.ReadFrom(params.Offset, params.Limit)
	if err != nil {
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return records, nil
}

func (s *Server) handleEventsStream(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.eventBus == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "event bus not initialized"}
	}

	log := s.eventBus.GetLog()
	if log == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "durable log not enabled"}
	}

	var params struct {
		Offset    uint64 `json:"offset"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	timeout := time.Duration(params.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Use long-polling
	lctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	records, err := log.WaitForEvents(lctx, params.Offset)
	if err != nil {
		if err == context.DeadlineExceeded {
			return map[string]interface{}{
				"events": []*eventlog.Record{},
				"count":  0,
			}, nil
		}
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{
		"events": records,
		"count":  len(records),
	}, nil
}

// handleStoreKey stores an API key in the keystore
func (s *Server) handleStoreKey(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		ID          string `json:"id"`
		Provider    string `json:"provider"`
		Token       string `json:"token"`
		DisplayName string `json:"display_name"`
		BaseURL     string `json:"base_url,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	if params.ID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "id is required"}
	}
	if params.Provider == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "provider is required"}
	}
	if params.Token == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "token is required"}
	}

	// Get keystore from server config
	if s.keystore == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "keystore not configured"}
	}

	ks, ok := s.keystore.(*keystore.Keystore)
	if !ok {
		return nil, &ErrorObj{Code: InternalError, Message: "keystore not available"}
	}

	if err := ks.Open(); err != nil {
		return nil, &ErrorObj{Code: InternalError, Message: "failed to open keystore: " + err.Error()}
	}
	defer ks.Close()

	cred := keystore.Credential{
		ID:          params.ID,
		Provider:    keystore.Provider(params.Provider),
		Token:       params.Token,
		BaseURL:     params.BaseURL,
		DisplayName: params.DisplayName,
		CreatedAt:   time.Now().Unix(),
	}

	if err := ks.Store(cred); err != nil {
		return nil, &ErrorObj{Code: InternalError, Message: "failed to store key: " + err.Error()}
	}

	if s.metrics != nil {
		s.metrics.IncrementCounter("armorclaw_keystore_operations_total", "store")
	}

	return map[string]interface{}{
		"success":      true,
		"id":           params.ID,
		"provider":     params.Provider,
		"display_name": params.DisplayName,
	}, nil
}

// handleProvisioningStart creates a new provisioning token
func (s *Server) handleProvisioningStart(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.provisioningMgr == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "provisioning not configured"}
	}

	// Generate a simple setup token (in production, use proper crypto)
	setupToken := fmt.Sprintf("setup_%d_%s", time.Now().Unix(), randomString(16))
	qrData := fmt.Sprintf("armorclaw://setup?token=%s", setupToken)

	return map[string]interface{}{
		"setup_token": setupToken,
		"qr_data":     qrData,
		"expires_in":  3600,
	}, nil
}

// handleProvisioningClaim claims a role for a device
func (s *Server) handleProvisioningClaim(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		SetupToken string `json:"setup_token"`
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name,omitempty"`
		DeviceType string `json:"device_type,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	if params.SetupToken == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "setup_token is required"}
	}
	if params.DeviceID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "device_id is required"}
	}

	// Determine role based on claim order (first claim = OWNER)
	// In production, use proper role management
	role := "USER"
	if strings.HasPrefix(params.DeviceID, "@admin:") {
		role = "OWNER"
	}

	return map[string]interface{}{
		"success":     true,
		"role":        role,
		"device_id":   params.DeviceID,
		"device_name": params.DeviceName,
	}, nil
}

// handleStudioStats returns Agent Studio statistics
func (s *Server) handleStudioStats(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	// Delegate to studio handler if available
	if s.studio != nil {
		studioResp := s.studio.HandleRPCMethod("studio.stats", req.Params)
		if studioResp.Error != nil {
			return nil, &ErrorObj{Code: studioResp.Error.Code, Message: studioResp.Error.Message}
		}
		return studioResp.Result, nil
	}

	// Return basic stats if studio not fully initialized
	return map[string]interface{}{
		"agents":    0,
		"instances": 0,
		"skills":    0,
		"mcps":      0,
		"status":    "initialized",
	}, nil
}

// randomString generates a random string of given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}

// keystoreFeatureDisabled returns a standard -32601 error when the zero-trust
// keystore feature flag is off.
func keystoreFeatureDisabled() *ErrorObj {
	return &ErrorObj{Code: MethodNotFound, Message: "Feature disabled: zero_trust_keystore"}
}

// identityFromCtx extracts a rate-limiting identity string from the RPC context.
// It mirrors resolveClientIdentity but works without *http.Request.
func identityFromCtx(ctx context.Context) string {
	if cred := PeerCredFromContext(ctx); cred != nil {
		return "uid:" + itoa(int64(cred.UID))
	}
	return "rpc:unknown"
}

func (s *Server) handleKeystoreUnseal(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	identity := identityFromCtx(ctx)
	if s.keystoreLimiter != nil && s.keystoreLimiter.Exceeded(identity) {
		return nil, &ErrorObj{Code: KeystoreRateLimited, Message: "rate limit exceeded"}
	}

	var params struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}
	if params.Password == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "password is required"}
	}

	if s.sealedKS == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "sealed keystore not configured"}
	}

	err := s.sealedKS.UnsealWithPassword(params.Password)
	if s.keystoreLimiter != nil {
		s.keystoreLimiter.Record(identity)
	}
	if err != nil {
		if pue, ok := err.(*keystore.PasswordUnsealError); ok {
			return nil, &ErrorObj{Code: pue.Code, Message: pue.Message}
		}
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	if s.auditLog != nil {
		_ = s.auditLog.LogEvent(audit.EventKeystoreUnseal, "", "", identity, nil)
	}

	return map[string]interface{}{"unsealed": true}, nil
}

func (s *Server) handleKeystoreSealed(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	if s.sealedKS == nil {
		return map[string]interface{}{"sealed": true}, nil
	}

	return map[string]interface{}{"sealed": !s.sealedKS.IsPasswordUnsealed()}, nil
}

func (s *Server) handleKeystoreSeal(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	if s.sealedKS == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "sealed keystore not configured"}
	}

	s.sealedKS.SealPassword()
	return map[string]interface{}{"sealed": true}, nil
}

func (s *Server) handleKeystoreExtendSession(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	if s.sealedKS == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "sealed keystore not configured"}
	}

	if err := s.sealedKS.ExtendPasswordSession(); err != nil {
		if pue, ok := err.(*keystore.PasswordUnsealError); ok {
			return nil, &ErrorObj{Code: pue.Code, Message: pue.Message}
		}
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{"extended": true}, nil
}

func (s *Server) handleKeystoreSessionStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	if s.sealedKS == nil {
		return map[string]interface{}{"sealed": true, "remaining_seconds": float64(0), "last_activity_at": ""}, nil
	}

	status := s.sealedKS.SessionStatus()
	return map[string]interface{}{
		"sealed":            status.Sealed,
		"remaining_seconds": status.RemainingSeconds,
		"last_activity_at":  status.LastActivityAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleKeystoreListKeys(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	if s.sealedKS == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "sealed keystore not configured"}
	}

	keys, err := s.sealedKS.ListKeys(ctx)
	if err != nil {
		if pue, ok := err.(*keystore.PasswordUnsealError); ok {
			return nil, &ErrorObj{Code: pue.Code, Message: pue.Message}
		}
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{"keys": keys}, nil
}

func (s *Server) handleKeystoreDeleteKey(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.zeroTrustKS {
		return nil, keystoreFeatureDisabled()
	}

	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}
	if params.Name == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "name is required"}
	}

	if s.sealedKS == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "sealed keystore not configured"}
	}

	if err := s.sealedKS.DeleteKey(ctx, params.Name); err != nil {
		if pue, ok := err.(*keystore.PasswordUnsealError); ok {
			return nil, &ErrorObj{Code: pue.Code, Message: pue.Message}
		}
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{"deleted": true}, nil
}

// voiceFeatureDisabled returns a standard -32601 error when the voice pipeline
// feature flag is off.
func voiceFeatureDisabled() *ErrorObj {
	return &ErrorObj{Code: MethodNotFound, Message: "Feature disabled: voice_pipeline"}
}

func (s *Server) handleVoiceStartSession(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.voicePipeline != "cloud" {
		return nil, voiceFeatureDisabled()
	}

	var params struct {
		SessionConfig json.RawMessage `json:"session_config"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	if s.voiceMgr == nil {
		return nil, &ErrorObj{Code: voice.ErrVoiceNotConfiguredCode, Message: "voice pipeline not configured: manager is nil"}
	}

	if failures := voice.CheckVoicePrereqs(s.matrix != nil && s.matrix.IsLoggedIn()); len(failures) > 0 {
		return nil, &ErrorObj{
			Code:    voice.ErrVoiceNotConfiguredCode,
			Message: "voice pipeline prerequisites not met",
			Data:    failures,
		}
	}

	calls := s.voiceMgr.ListCalls()
	sessionID := fmt.Sprintf("voice-%d", len(calls)+1)

	return map[string]interface{}{"session_id": sessionID}, nil
}

func (s *Server) handleVoiceStopSession(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.voicePipeline != "cloud" {
		return nil, voiceFeatureDisabled()
	}

	var params struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	if s.voiceMgr == nil {
		return nil, &ErrorObj{Code: voice.ErrVoiceNotConfiguredCode, Message: "voice pipeline not configured: manager is nil"}
	}

	if failures := voice.CheckVoicePrereqs(s.matrix != nil && s.matrix.IsLoggedIn()); len(failures) > 0 {
		return nil, &ErrorObj{
			Code:    voice.ErrVoiceNotConfiguredCode,
			Message: "voice pipeline prerequisites not met",
			Data:    failures,
		}
	}

	if params.SessionID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "session_id is required"}
	}

	if call, ok := s.voiceMgr.GetCall(params.SessionID); ok {
		_ = s.voiceMgr.EndCall(call.ID, "rpc_stop_session")
	}

	return map[string]interface{}{"stopped": true}, nil
}

func (s *Server) handleVoiceIntentSubmit(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.voicePipeline != "edge" && s.voicePipeline != "cloud" {
		return nil, voiceFeatureDisabled()
	}

	var params voice.VoiceIntentRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	// Reject raw audio payloads — edge voice mode accepts text transcripts only
	var rawCheck map[string]json.RawMessage
	if err := json.Unmarshal(req.Params, &rawCheck); err == nil {
		if _, hasAudio := rawCheck["audio"]; hasAudio {
			return nil, &ErrorObj{
				Code:    InvalidParams,
				Message: "raw audio not accepted in edge voice mode: send transcript text only",
			}
		}
		if _, hasAudioData := rawCheck["audio_data"]; hasAudioData {
			return nil, &ErrorObj{
				Code:    InvalidParams,
				Message: "raw audio not accepted in edge voice mode: send transcript text only",
			}
		}
	}

	resp, logEntry, err := voice.ProcessVoiceIntent(ctx, &params)
	if err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: err.Error()}
	}

	if s.auditLog != nil && logEntry != nil {
		_ = s.auditLog.LogEvent(audit.EventVoiceStartSession, logEntry.SessionID, "", "", map[string]interface{}{
			"source":      logEntry.Source,
			"transcript":  logEntry.Transcript,
			"confidence":  logEntry.Confidence,
			"locale":      logEntry.Locale,
			"action":      logEntry.ActionTaken,
			"timestamp":   logEntry.Timestamp,
		})
	}

	return resp, nil
}

func (s *Server) handleVoiceStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.voicePipeline != "cloud" {
		return nil, voiceFeatureDisabled()
	}

	if s.voiceMgr == nil {
		return nil, &ErrorObj{Code: voice.ErrVoiceNotConfiguredCode, Message: "voice pipeline not configured: manager is nil"}
	}

	if failures := voice.CheckVoicePrereqs(s.matrix != nil && s.matrix.IsLoggedIn()); len(failures) > 0 {
		return nil, &ErrorObj{
			Code:    voice.ErrVoiceNotConfiguredCode,
			Message: "voice pipeline prerequisites not met",
			Data:    failures,
		}
	}

	calls := s.voiceMgr.ListCalls()
	sessions := make([]map[string]interface{}, 0, len(calls))
	for _, c := range calls {
		sessions = append(sessions, map[string]interface{}{
			"id":      c.ID,
			"room_id": c.RoomID,
			"state":   c.State,
		})
	}

	return map[string]interface{}{
		"active":   len(calls) > 0,
		"sessions": sessions,
	}, nil
}

func (s *Server) registerHandlers() {
	h := map[string]HandlerFunc{
		"ai.chat":                    s.handleAIChat,
		"browser.navigate":           s.handleBrowserNavigate,
		"browser.fill":               s.handleBrowserFill,
		"browser.click":              s.handleBrowserClick,
		"browser.status":             s.handleBrowserStatus,
		"browser.wait_for_element":   s.handleBrowserWaitForElement,
		"browser.wait_for_captcha":   s.handleBrowserWaitForCaptcha,
		"browser.wait_for_2fa":       s.handleBrowserWaitFor2FA,
		"browser.complete":           s.handleBrowserComplete,
		"browser.fail":               s.handleBrowserFail,
		"browser.list":               s.handleBrowserList,
		"browser.cancel":             s.handleBrowserCancel,
		"browser.replay_diagnostics": s.handleBrowserReplayDiagnostics,
		"browser.screenshot":         s.handleBrowserScreenshot,
		"browser.close":              s.handleBrowserClose,
		"bridge.start":               s.handleBridgeStart,
		"bridge.stop":                s.handleBridgeStop,
		"bridge.status":              s.handleBridgeStatus,
		"bridge.channel":             s.handleBridgeChannel,
		"bridge.unchannel":           s.handleUnbridgeChannel,
		"bridge.list":                s.handleListBridgedChannels,
		"bridge.ghost_list":          s.handleGhostUserList,
		"bridge.appservice_status":   s.handleAppServiceStatus,
		"pii.request":                s.handlePIIRequest,
		"pii.approve":                s.handlePIIApprove,
		"pii.deny":                   s.handlePIIDeny,
		"pii.status":                 s.handlePIIStatus,
		"pii.list_pending":           s.handlePIIListPending,
		"pii.stats":                  s.handlePIIStats,
		"pii.cancel":                 s.handlePIICancel,
		"pii.fulfill":                s.handlePIIFulfill,
		"pii.wait_for_approval":      s.handlePIIWaitForApproval,
		"skills.execute":             s.handleSkillsExecute,
		"skills.list":                s.handleSkillsList,
		"skills.get_schema":          s.handleSkillsGetSchema,
		"skills.allow":               s.handleSkillsAllow,
		"skills.block":               s.handleSkillsBlock,
		"skills.allowlist_add":       s.handleSkillsAllowlistAdd,
		"skills.allowlist_remove":    s.handleSkillsAllowlistRemove,
		"skills.allowlist_list":      s.handleSkillsAllowlistList,
		"skills.web_search":          s.handleSkillsWebSearch,
		"skills.web_extract":         s.handleSkillsWebExtract,
		"skills.email_send":          s.handleSkillsEmailSend,
		"skills.slack_message":       s.handleSkillsSlackMessage,
		"skills.file_read":           s.handleSkillsFileRead,
		"skills.data_analyze":        s.handleSkillsDataAnalyze,
		"matrix.status":              s.handleMatrixStatus,
		"matrix.login":               s.handleMatrixLogin,
		"matrix.send":                s.handleMatrixSend,
		"matrix.receive":             s.handleMatrixReceive,
		"matrix.join_room":           s.handleMatrixJoinRoom,
		"events.replay":              s.handleEventsReplay,
		"events.stream":              s.handleEventsStream,
		// Agent Studio methods (delegation gate applied internally by handleStudio)
		"studio.list_skills":                   s.handleStudio,
		"studio.get_skill":                     s.handleStudio,
		"studio.register_skill":                s.handleStudio,
		"studio.list_pii":                      s.handleStudio,
		"studio.get_pii":                       s.handleStudio,
		"studio.register_pii":                  s.handleStudio,
		"studio.list_profiles":                 s.handleStudio,
		"studio.create_agent":                  s.handleStudio,
		"studio.update_agent":                  s.handleStudio,
		"studio.delete_agent":                  s.handleStudio,
		"studio.list_agents":                   s.handleStudio,
		"studio.get_agent":                     s.handleStudio,
		"studio.spawn_agent":                   s.handleStudio,
		"studio.list_instances":                s.handleStudio,
		"studio.get_instance":                  s.handleStudio,
		"studio.stop_instance":                 s.handleStudio,
		"studio.stats":                         s.handleStudioStats,
		"studio.list_mcps":                     s.handleStudio,
		"studio.get_mcp":                       s.handleStudio,
		"studio.get_mcp_warning":               s.handleStudio,
		"studio.request_mcp_approval":          s.handleStudio,
		"studio.list_pending_approvals":        s.handleStudio,
		"studio.list_my_approvals":             s.handleStudio,
		"studio.approve_mcp_request":           s.handleStudio,
		"studio.reject_mcp_request":            s.handleStudio,
		"store_key":                            s.handleStoreKey,
		"provisioning.start":                   s.handleProvisioningStart,
		"provisioning.claim":                   s.handleProvisioningClaim,
		"hardening.status":                     s.handleHardeningStatus,
		"hardening.ack":                        s.handleHardeningAck,
		"hardening.rotate_password":            s.handleHardeningRotatePassword,
		"health.check":                         s.handleHealthCheck,
		"mobile.heartbeat":                     s.handleMobileHeartbeat,
		"container.terminate":                  s.handleTerminateContainer,
		"container.list":                       s.handleListContainers,
		"resolve_blocker":                      s.handleResolveBlocker,
		"approve_email":                        s.handleApproveEmail,
		"deny_email":                           s.handleDenyEmail,
		"email_approval_status":                s.handleEmailApprovalStatus,
		"email.list_pending":                   s.handleEmailListPending,
		"account.delete":                       s.handleAccountDelete,
		"secretary.start_workflow":             s.handleSecretaryMethod,
		"secretary.get_workflow":               s.handleSecretaryMethod,
		"secretary.cancel_workflow":            s.handleSecretaryMethod,
		"secretary.create_workflow":            s.handleSecretaryMethod,
		"secretary.advance_workflow":           s.handleSecretaryMethod,
		"secretary.list_templates":             s.handleSecretaryMethod,
		"secretary.create_template":            s.handleSecretaryMethod,
		"secretary.get_template":               s.handleSecretaryMethod,
		"secretary.delete_template":            s.handleSecretaryMethod,
		"secretary.update_template":            s.handleSecretaryMethod,
		"secretary.is_running":                 s.handleSecretaryMethod,
		"secretary.get_active_count":           s.handleSecretaryMethod,
		"secretary.shutdown":                   s.handleSecretaryMethod,
		"secretary.artifact_upload":            s.handleArtifactMethod,
		"secretary.artifact_download":          s.handleArtifactMethod,
		"secretary.artifact_list":              s.handleArtifactMethod,
		"secretary.artifact_update_status":     s.handleArtifactMethod,
		"task.create":                          s.handleSecretaryMethod,
		"task.list":                            s.handleSecretaryMethod,
		"task.cancel":                          s.handleSecretaryMethod,
		"task.get":                             s.handleSecretaryMethod,
		"secretary.set_approval_delegation":    s.handleSecretaryMethod,
		"secretary.get_approval_delegation":    s.handleSecretaryMethod,
		"secretary.revoke_approval_delegation": s.handleSecretaryMethod,
		"device.list":                          s.handleDeviceList,
		"device.get":                           s.handleDeviceGet,
		"device.approve":                       s.handleDeviceApprove,
		"device.reject":                        s.handleDeviceReject,
		"invite.list":                          s.handleInviteList,
		"invite.create":                        s.handleInviteCreate,
		"invite.revoke":                        s.handleInviteRevoke,
		"invite.validate":                      s.handleInviteValidate,
		"bridge.e2ee_enable":                   s.handleE2EEEnable,
		"bridge.e2ee_disable":                  s.handleE2EEDisable,
		"keystore.unseal":                      s.handleKeystoreUnseal,
		"keystore.sealed":                      s.handleKeystoreSealed,
		"keystore.seal":                        s.handleKeystoreSeal,
		"keystore.extend_session":              s.handleKeystoreExtendSession,
		"keystore.session_status":              s.handleKeystoreSessionStatus,
		"keystore.list_keys":                   s.handleKeystoreListKeys,
		"keystore.delete_key":                  s.handleKeystoreDeleteKey,
		"voice.start_session":                  s.handleVoiceStartSession,
		"voice.stop_session":                   s.handleVoiceStopSession,
		"voice.status":                         s.handleVoiceStatus,
		"voice.intent.submit":                  s.handleVoiceIntentSubmit,
		"e2ee.create_backup":                   s.handleE2EECreateBackup,
		"e2ee.delete_backup":                   s.handleE2EEDeleteBackup,
		"e2ee.backup_exists":                   s.handleE2EEBackupExists,
		"document.extract_text":                s.handleDocumentExtractText,
		"document.status":                      s.handleDocumentStatus,
		"document.list_jobs":                   s.handleDocumentListJobs,
		"email.queue_status":                   s.handleEmailQueueStatus,
		"email.get":                            s.handleEmailGet,
		"email.retry":                          s.handleEmailRetry,
		"email.list":                           s.handleEmailList,
	}

	s.handlers = h

	// Wrap BEATO-sensitive handlers with SafetyMiddleware.
	if s.safety != nil {
		beatoHandlers := map[string]RPCGroup{
			// Browser group
			"browser.navigate":           BrowserRPCGroup,
			"browser.fill":               BrowserRPCGroup,
			"browser.click":              BrowserRPCGroup,
			"browser.status":             BrowserRPCGroup,
			"browser.wait_for_element":   BrowserRPCGroup,
			"browser.wait_for_captcha":   BrowserRPCGroup,
			"browser.wait_for_2fa":       BrowserRPCGroup,
			"browser.complete":           BrowserRPCGroup,
			"browser.fail":               BrowserRPCGroup,
			"browser.list":               BrowserRPCGroup,
			"browser.cancel":             BrowserRPCGroup,
			"browser.replay_diagnostics": BrowserRPCGroup,
			"browser.screenshot":         BrowserRPCGroup,
			"browser.close":              BrowserRPCGroup,
			// Document group
			"document.extract_text": DocumentRPCGroup,
			"document.status":       DocumentRPCGroup,
			"document.list_jobs":    DocumentRPCGroup,
			// Email group
			"email.queue_status":    EmailRPCGroup,
			"email.get":             EmailRPCGroup,
			"email.retry":           EmailRPCGroup,
			"email.list":            EmailRPCGroup,
			"approve_email":         EmailRPCGroup,
			"deny_email":            EmailRPCGroup,
			"email_approval_status": EmailRPCGroup,
			"email.list_pending":    EmailRPCGroup,
		}
		for method, group := range beatoHandlers {
			if handler, ok := h[method]; ok {
				h[method] = s.safety.WrapForGroup(group, handler)
			}
		}
	}
}

func (s *Server) handleHardeningStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	matrixAdapter, ok := s.matrix.(*adapter.MatrixAdapter)
	if !ok || matrixAdapter == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Matrix adapter not configured",
		}
	}

	hardeningHandler := NewHardeningHandler(s.hardeningStore, matrixAdapter, "/var/lib/armorclaw/.admin_password")
	return hardeningHandler.handleHardeningStatus(ctx, req)
}

func (s *Server) handleHardeningAck(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	matrixAdapter, ok := s.matrix.(*adapter.MatrixAdapter)
	if !ok || matrixAdapter == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Matrix adapter not configured",
		}
	}

	hardeningHandler := NewHardeningHandler(s.hardeningStore, matrixAdapter, "/var/lib/armorclaw/.admin_password")
	return hardeningHandler.handleHardeningAck(ctx, req)
}

func (s *Server) handleHardeningRotatePassword(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	matrixAdapter, ok := s.matrix.(*adapter.MatrixAdapter)
	if !ok || matrixAdapter == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Matrix adapter not configured",
		}
	}

	hardeningHandler := NewHardeningHandler(s.hardeningStore, matrixAdapter, "/var/lib/armorclaw/.admin_password")
	return hardeningHandler.handleHardeningRotatePassword(ctx, req)
}

func (s *Server) handleResolveBlocker(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		WorkflowID string `json:"workflow_id"`
		StepID     string `json:"step_id"`
		Input      string `json:"input"`
		Note       string `json:"note"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: "invalid parameters"}
	}

	// Validate required fields
	if params.WorkflowID == "" || params.StepID == "" || params.Input == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "workflow_id, step_id, and input are required"}
	}

	// Build response — PII SAFETY: Input is NEVER logged (intentional omission)
	response := secretary.BlockerResponse{
		Input:      params.Input,
		Note:       params.Note,
		UserID:     "", // extracted from auth context in production
		ProvidedAt: time.Now().Unix(),
	}

	delivered := secretary.DeliverBlockerResponse(params.WorkflowID, params.StepID, response)
	if !delivered {
		return nil, &ErrorObj{Code: InvalidParams, Message: "no pending blocker for workflow " + params.WorkflowID + " step " + params.StepID}
	}

	return map[string]interface{}{"status": "delivered"}, nil
}

// GetMatrixAdapter returns the Matrix adapter for external integration
func (s *Server) GetMatrixAdapter() MatrixAdapter {
	return s.matrix
}

// GetLastHeartbeat returns the last heartbeat timestamp for a user
func (s *Server) GetLastHeartbeat(userID string) time.Time {
	if userID == "" {
		return time.Time{}
	}
	if val, ok := s.heartbeats.Load(userID); ok {
		if ts, ok := val.(time.Time); ok {
			return ts
		}
	}
	return time.Time{}
}

func (s *Server) Run(socketPath string) error {
	var listener net.Listener
	var err error

	useTCP := false
	listenAddr := socketPath

	if s.rpcTransport == "tcp" && s.listenAddr != "" {
		useTCP = true
		listenAddr = s.listenAddr
	}

	if !useTCP && runtime.GOOS == "windows" {
		useTCP = true
		listenAddr = "127.0.0.1:6168"
	}

	if useTCP {
		listener, err = net.Listen("tcp", listenAddr)
		if err != nil {
			return fmt.Errorf("failed to create TCP listener on %s: %w", listenAddr, err)
		}
		if s.safety != nil && s.safety.cfg.AdminToken != "" {
			slog.Info("RPC server listening", "transport", "tcp", "address", listenAddr, "tcp_auth", "enabled")
		} else {
			slog.Warn("RPC server listening", "transport", "tcp", "address", listenAddr, "tcp_auth", "disabled", "reason", "no AdminToken")
		}
	} else {
		socketDir := filepath.Dir(listenAddr)
		if err := os.MkdirAll(socketDir, 0755); err != nil {
			return fmt.Errorf("failed to create socket directory: %w", err)
		}

		if _, err := os.Stat(listenAddr); err == nil {
			os.Remove(listenAddr)
		}

		listener, err = net.Listen("unix", listenAddr)
		if err != nil {
			return fmt.Errorf("failed to create Unix socket at %s: %w", listenAddr, err)
		}

		if err := os.Chmod(listenAddr, 0660); err != nil {
			slog.Warn("failed to set socket permissions", "error", err)
		}

		slog.Info("RPC server listening", "transport", "unix", "path", listenAddr)
	}

	s.listener = listener

	// Note: Signal handling is consolidated in main.go.
	// When main.go's signal handler fires, the process exits and the
	// listener is closed by the OS, causing Accept() to return an error.
	for {
		conn, err := listener.Accept()
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			return fmt.Errorf("accept error: %w", err)
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	if s.guard != nil {
		if err := s.guard.Check(conn.RemoteAddr()); err != nil {
			slog.Warn("trusted_proxy_guard_rejected", "error", err)
			conn.Close()
			return
		}
	}

	defer conn.Close()

	br := bufio.NewReader(conn)

	// Intercept HTTP requests before JSON-RPC decode
	if peek, err := br.Peek(20); err == nil {
		peekStr := string(peek)
		if strings.HasPrefix(peekStr, "GET /health ") || strings.HasPrefix(peekStr, "GET /health?") {
			// Consume the full HTTP request headers
			for {
				line, err := br.ReadString('\n')
				if err != nil || line == "\r\n" || line == "\n" {
					break
				}
			}
			fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{\"status\":\"ok\"}\n")
			return
		}
		if strings.HasPrefix(peekStr, "GET ") {
			// Consume the full HTTP request headers
			for {
				line, err := br.ReadString('\n')
				if err != nil || line == "\r\n" || line == "\n" {
					break
				}
			}
			fmt.Fprintf(conn, "HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{\"status\":\"not_found\"}\n")
			return
		}
		if strings.HasPrefix(peekStr, "POST /setup") {
			// Consume the full HTTP request (headers + body)
			for {
				line, err := br.ReadString('\n')
				if err != nil || line == "\r\n" || line == "\n" {
					break
				}
			}
			// Skip any remaining body bytes (POST /setup has no required body)

			if s.provisioningMgr == nil {
				fmt.Fprintf(conn, "HTTP/1.1 503 Service Unavailable\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{\"status\":\"unavailable\",\"error\":\"provisioning not configured\"}\n")
				return
			}

			qrLink, expiresAt, err := s.provisioningMgr.StartSetupToken(context.Background())
			if err != nil {
				fmt.Fprintf(conn, "HTTP/1.1 500 Internal Server Error\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{\"status\":\"error\",\"error\":\"%s\"}\n", err.Error())
				return
			}

			resp := fmt.Sprintf(`{"status":"ok","admin_created":true,"deep_link":"%s","expires_at":"%s"}`, qrLink, expiresAt)
			fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n%s\n", resp)
			return
		}
		if strings.HasPrefix(peekStr, "POST /api") {
			s.handleHTTPAPI(conn, br)
			return
		}
	}

	// Read JSON-RPC request
	var req Request
	decoder := json.NewDecoder(br)
	if err := decoder.Decode(&req); err != nil {
		slog.Warn("rpc_decode_error", "error", err)
		return
	}

	// Handle request
	resp := s.Handle(context.Background(), &req)

	// Write response
	encoder := json.NewEncoder(conn)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(resp); err != nil {
		slog.Warn("rpc_write_error", "error", err)
	}
}

func (s *Server) handleHTTPAPI(conn net.Conn, br *bufio.Reader) {
	var bearerToken string
	for {
		line, err := br.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Authorization:") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 2 && strings.EqualFold(parts[1], "Bearer") && len(parts) == 3 {
				bearerToken = parts[2]
			}
		}
	}

	var req Request
	if err := json.NewDecoder(br).Decode(&req); err != nil {
		fmt.Fprintf(conn, "HTTP/1.1 400 Bad Request\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":null,\"error\":{\"code\":-32700,\"message\":\"parse error\"}}\n")
		return
	}
	req.AuthToken = bearerToken

	resp := s.Handle(context.Background(), &req)

	respBytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(conn, "HTTP/1.1 500 Internal Server Error\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{\"error\":\"marshal failed\"}\n")
		return
	}
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: %d\r\n\r\n%s\n", len(respBytes)+1, string(respBytes))
}
