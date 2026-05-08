// Package voice provides the voice call manager
// Integrates WebRTC, Matrix, budget, and security for voice calls
package voice

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/turn"
	"github.com/armorclaw/bridge/pkg/webrtc"
	"github.com/armorclaw/bridge/pkg/logger"
	"log/slog"
)

// NOTE: Voice manager initialization is commented out in main.go. voiceMgr remains nil at runtime.
// Manager integrates all voice call components
type Manager struct {
	// Core components
	config      ManagerConfig
	sessionMgr  *webrtc.SessionManager
	tokenMgr    *webrtc.TokenManager
	webrtcEngine *webrtc.Engine
	turnMgr     *turn.Manager

	// Voice-specific components
	voiceMgr    *MatrixManager
	budgetTracker *BudgetTracker
	securityEnforcer *SecurityEnforcer
	ttlManager  *TTLManager
	securityAudit *SecurityAudit

	// PCM audio routing
	pcmRouters sync.Map // map[callID]*PCMRouter

	// State
	calls       sync.Map // map[callID]*MatrixCall
	activeCount int
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	securityLog *logger.SecurityLogger
}

// ManagerConfig holds configuration for the voice manager
type ManagerConfig struct {
	// WebRTC configuration
	WebRTCConfig webrtc.EngineConfig

	// Voice configuration
	VoiceConfig Config

	// Security policy
	SecurityPolicy SecurityPolicy

	// Budget configuration
	BudgetConfig BudgetConfig

	// Session management
	DefaultLifetime time.Duration
	MaxLifetime     time.Duration

	// TURN configuration
	TURNSharedSecret string
	TURNServerURL    string

	// Call limits
	MaxConcurrentCalls int
	MaxCallDuration    time.Duration

	// Budget defaults (for StartCall)
	DefaultTokenLimit    uint64
	DefaultDurationLimit time.Duration
}

// BudgetConfig holds budget configuration for voice manager
type BudgetConfig struct {
	DefaultTokenLimit    uint64
	DefaultDurationLimit time.Duration
	WarningThreshold     float64
	HardStop             bool
}

// VoiceConfig holds voice-specific configuration (aliased for clarity)
type VoiceConfig = Config

// NewManager creates a new voice manager
func NewManager(
	sessionMgr *webrtc.SessionManager,
	tokenMgr *webrtc.TokenManager,
	webrtcEngine *webrtc.Engine,
	turnMgr *turn.Manager,
	config ManagerConfig,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Create security enforcer
	securityEnforcer := NewSecurityEnforcer(config.SecurityPolicy)

	// Create budget tracker
	budgetTracker := NewBudgetTracker(Config{
		DefaultTokenLimit:    config.BudgetConfig.DefaultTokenLimit,
		DefaultDurationLimit: config.BudgetConfig.DefaultDurationLimit,
		WarningThreshold:     config.BudgetConfig.WarningThreshold,
		HardStop:             config.BudgetConfig.HardStop,
	})

	// Create TTL manager
	ttlConfig := DefaultTTLConfig()
	ttlManager := NewTTLManager(sessionMgr, ttlConfig)

	// Create security auditor
	securityAudit := NewSecurityAudit(config.SecurityPolicy)

	return &Manager{
		config:         config,
		sessionMgr:     sessionMgr,
		tokenMgr:       tokenMgr,
		webrtcEngine:   webrtcEngine,
		turnMgr:        turnMgr,
		voiceMgr:       nil, // MatrixManager stubbed out
		budgetTracker:  budgetTracker,
		securityEnforcer: securityEnforcer,
		ttlManager:     ttlManager,
		securityAudit:  securityAudit,
		ctx:            ctx,
		cancel:         cancel,
		securityLog:    logger.NewSecurityLogger(logger.Global().WithComponent("voice_manager")),
	}
}

// Start starts the voice manager
func (m *Manager) Start() error {
	// Start TTL enforcement
	if err := m.ttlManager.StartEnforcement(m.ctx); err != nil {
		return fmt.Errorf("failed to start TTL enforcement: %w", err)
	}

	// Start budget enforcement
	if err := m.budgetTracker.StartEnforcement(m.ctx); err != nil {
		return fmt.Errorf("failed to start budget enforcement: %w", err)
	}

	// Start voice manager (if configured)
	if m.voiceMgr != nil {
		m.voiceMgr.Start()
	}

	m.securityLog.LogSecurityEvent("voice_manager_started",
		slog.Int("max_concurrent_calls", m.config.MaxConcurrentCalls),
		slog.Duration("max_call_duration", m.config.MaxCallDuration),
	)

	return nil
}

// Stop stops the voice manager
func (m *Manager) Stop() {
	// Signal all goroutines to stop
	m.cancel()

	// Stop voice manager (if configured)
	if m.voiceMgr != nil {
		m.voiceMgr.Stop()
	}

	// Wait for all goroutines
	m.wg.Wait()

	// Clean up all active calls
	m.calls.Range(func(key, value interface{}) bool {
		call, ok := value.(*MatrixCall)
		if ok {
			m.EndCall(call.ID, "manager_shutdown")
		}
		return true
	})

	m.securityLog.LogSecurityEvent("voice_manager_stopped")
}

// HandleMatrixCallEvent handles a Matrix call event
func (m *Manager) HandleMatrixCallEvent(roomID, eventID, senderID string, event *CallEvent) error {
	// Forward to voice manager (if configured)
	if m.voiceMgr == nil {
		return fmt.Errorf("voice manager not configured")
	}
	return m.voiceMgr.HandleCallEvent(roomID, eventID, senderID, event)
}

// CreateCall creates a new voice call
func (m *Manager) CreateCall(roomID, offerSDP string, userID string) (*MatrixCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if voice manager is configured
	if m.voiceMgr == nil {
		return nil, fmt.Errorf("voice manager not configured")
	}

	// Check concurrent call limit
	activeCount := m.getActiveCallCount()
	if activeCount >= m.config.MaxConcurrentCalls {
		m.securityLog.LogAccessDenied(m.ctx, "voice_call_create", roomID, "max_concurrent_calls_exceeded",
			slog.String("user_id", userID),
			slog.Int("max_calls", m.config.MaxConcurrentCalls))
		return nil, ErrMaxConcurrentCallsExceeded
	}

	// Check security policy
	if m.securityEnforcer != nil {
		if err := m.securityEnforcer.CheckStartCall(userID, roomID); err != nil {
			return nil, err
		}
	}

	// Create the call via voice manager
	call, err := m.voiceMgr.StartCall(roomID, offerSDP)
	if err != nil {
		return nil, err
	}

	// Store in active calls
	m.calls.Store(call.ID, call)
	m.activeCount++

	// Register with security enforcer
	m.securityEnforcer.RegisterCall(roomID, userID)

	// Create budget session
	budgetSession, err := m.budgetTracker.StartSession(
		call.ID,
		call.ID,
		roomID,
		m.config.DefaultTokenLimit,
		m.config.DefaultDurationLimit,
	)
	if err != nil {
		// Log but don't fail - budget is optional
		m.securityLog.LogSecurityEvent("voice_budget_session_failed",
			slog.String("call_id", call.ID),
			slog.String("error", err.Error()))
	} else {
		call.mu.Lock()
		call.BudgetSession = budgetSession
		call.mu.Unlock()
	}

	m.securityLog.LogSecurityEvent("voice_call_created",
		slog.String("call_id", call.ID),
		slog.String("room_id", roomID),
		slog.String("user_id", userID),
		slog.Int("active_calls", m.activeCount))

	return call, nil
}

// AnswerCall answers an incoming call
func (m *Manager) AnswerCall(callID, answerSDP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the call
	value, ok := m.calls.Load(callID)
	if !ok {
		return ErrCallNotFound
	}

	matrixCall, ok := value.(*MatrixCall)
	if !ok {
		return fmt.Errorf("invalid call type for %s", callID)
	}

	// Answer via voice manager (if configured)
	if m.voiceMgr != nil {
		if err := m.voiceMgr.AnswerCall(callID, answerSDP); err != nil {
			return err
		}
	}

	matrixCall.mu.Lock()
	matrixCall.State = CallStateConnected
	matrixCall.AnsweredAt = time.Now()
	matrixCall.mu.Unlock()

	m.securityLog.LogSecurityEvent("voice_call_answered",
		slog.String("call_id", callID))

	return nil
}

// RejectCall rejects an incoming call
func (m *Manager) RejectCall(callID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the call
	_, ok := m.calls.Load(callID)
	if !ok {
		return ErrCallNotFound
	}

	// Reject via voice manager
	if err := m.voiceMgr.RejectCall(callID, reason); err != nil {
		return err
	}

	// Clean up
	m.cleanupCall(callID)

	m.securityLog.LogSecurityEvent("voice_call_rejected",
		slog.String("call_id", callID),
		slog.String("reason", reason))

	return nil
}

// EndCall ends an active call
func (m *Manager) EndCall(callID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the call
	value, ok := m.calls.Load(callID)
	if !ok {
		return ErrCallNotFound
	}

	call, ok := value.(*MatrixCall)
	if !ok {
		return fmt.Errorf("invalid call type for %s", callID)
	}

	// End via voice manager (if configured)
	if m.voiceMgr != nil {
		if err := m.voiceMgr.EndCall(callID, reason); err != nil {
			return err
		}
	}

	// End budget session
	m.budgetTracker.EndSession(callID)

	// Unregister from security
	m.securityEnforcer.UnregisterCall(call.RoomID, call.CallerID)

	// Audit the call
	_, err := m.securityAudit.AuditCall(call)
	if err != nil {
		m.securityLog.LogSecurityEvent("voice_call_audit_failed",
			slog.String("call_id", callID),
			slog.String("error", err.Error()))
	}

	// Clean up
	m.cleanupCall(callID)

	m.securityLog.LogSecurityEvent("voice_call_ended",
		slog.String("call_id", callID),
		slog.String("reason", reason))

	return nil
}

// SendCandidates sends ICE candidates for a call
func (m *Manager) SendCandidates(callID string, candidates []Candidate) error {
	if m.voiceMgr == nil {
		return fmt.Errorf("voice manager not configured")
	}
	return m.voiceMgr.SendCandidates(callID, candidates)
}

// GetCall retrieves an active call
func (m *Manager) GetCall(callID string) (*MatrixCall, bool) {
	value, ok := m.calls.Load(callID)
	if !ok {
		return nil, false
	}
	call, ok := value.(*MatrixCall)
	return call, ok
}

// ListCalls returns all active calls
func (m *Manager) ListCalls() []*MatrixCall {
	calls := make([]*MatrixCall, 0)

	m.calls.Range(func(key, value interface{}) bool {
		call, ok := value.(*MatrixCall)
		if ok {
			calls = append(calls, call)
		}
		return true
	})

	return calls
}

// GetStats returns voice manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})

	// Get base stats from voice manager (if configured)
	if m.voiceMgr != nil {
		stats = m.voiceMgr.GetStats()
	}

	// Add additional stats
	stats["active_calls"] = m.activeCount
	stats["max_concurrent_calls"] = m.config.MaxConcurrentCalls

	// Add budget stats
	if m.budgetTracker != nil {
		budgetStats := m.budgetTracker.GetStats()
		for k, v := range budgetStats {
			stats["budget_"+k] = v
		}
	}

	// Add TTL stats
	if m.ttlManager != nil {
		ttlStats := m.ttlManager.GetTTLStats()
		for k, v := range ttlStats {
			stats["ttl_"+k] = v
		}
	}

	return stats
}

// GetAuditLog returns the security audit log
func (m *Manager) GetAuditLog() []AuditRecord {
	return m.securityAudit.GetAuditLog()
}

// GenerateReport generates a security report
func (m *Manager) GenerateReport() *SecurityReport {
	return m.securityAudit.GenerateReport()
}

// cleanupCall removes a call from tracking
func (m *Manager) cleanupCall(callID string) {
	if router, ok := m.pcmRouters.LoadAndDelete(callID); ok {
		router.(*PCMRouter).Close()
	}
	m.calls.Delete(callID)
	m.activeCount--
}

// getActiveCallCount returns the number of active calls
func (m *Manager) getActiveCallCount() int {
	count := 0
	m.calls.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// CreateWebRTCSession creates a WebRTC session for a call
func (m *Manager) CreateWebRTCSession(callID, roomID string, ttl time.Duration) (*webrtc.Session, string, error) {
	// Create session
	session, err := m.sessionMgr.Create(
		fmt.Sprintf("container-%s", callID),
		roomID,
		ttl,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	// Generate call token
	token, err := m.tokenMgr.Generate(session.ID, roomID)
	if err != nil {
		m.sessionMgr.End(session.ID)
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Return session and token signature as the token string
	return session, token.Signature, nil
}

// GetWebRTCSession retrieves a WebRTC session by call ID
func (m *Manager) GetWebRTCSession(callID string) (*webrtc.Session, bool) {
	call, ok := m.GetCall(callID)
	if !ok {
		return nil, false
	}

	sessionID := call.GetSession()
	if sessionID == "" {
		return nil, false
	}

	return m.sessionMgr.Get(sessionID)
}

// ValidateCallToken validates a call session token
func (m *Manager) ValidateCallToken(token *webrtc.Token) (*webrtc.TokenClaims, error) {
	return m.tokenMgr.Validate(token)
}

// SetTURNManager sets the TURN credential manager
func (m *Manager) SetTURNManager(turnMgr *turn.Manager) {
	m.turnMgr = turnMgr
	// Note: webrtcEngine may not have SetTURNManager, this is for future implementation
}

// CreatePCMRouter creates a PCM audio router for a call session.
// The router bridges: Input PCM → VAD → STT → text → agent → text → TTS → output PCM.
// Audio never reaches agent containers (NetworkMode: none).
func (m *Manager) CreatePCMRouter(callID string, stt Transcriber, tts Synthesizer, agent AgentTextBridge) *PCMRouter {
	config := PCMRouterConfig{
		SampleRate: 16000, // 16kHz standard voice sample rate
		VADConfig: EnergyVADConfig{
			EnergyThreshold:    0.01,
			FrameDurationMs:    20,
			SilenceDurationMs:  300,
			SampleRate:         16000,
		},
		VADEnabled: true,
	}

	router := NewPCMRouter(config, stt, tts, agent)
	m.pcmRouters.Store(callID, router)
	return router
}

// GetPCMRouter retrieves the PCM router for a call
func (m *Manager) GetPCMRouter(callID string) (*PCMRouter, bool) {
	val, ok := m.pcmRouters.Load(callID)
	if !ok {
		return nil, false
	}
	return val.(*PCMRouter), true
}
