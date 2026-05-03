// Package qr provides QR code generation for remote setup support.
// This allows users to set up ArmorClaw from anywhere using public URLs.
package qr

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/securerandom"
	"github.com/skip2/go-qrcode"
)

// TokenType represents the type of one-time token
type TokenType string

const (
	TokenTypeSetup        TokenType = "setup"
	TokenTypeBonding      TokenType = "bonding"
	TokenTypeAPISecret    TokenType = "api_secret"
	TokenTypeInvite       TokenType = "invite"
	TokenTypeVerification TokenType = "verification"
	TokenTypeConfig       TokenType = "config" // Client configuration (ArmorTerminal/ArmorChat)
)

// TokenState represents the state of a one-time token
type TokenState string

const (
	TokenStateActive  TokenState = "active"
	TokenStateUsed    TokenState = "used"
	TokenStateExpired TokenState = "expired"
	TokenStateRevoked TokenState = "revoked"
)

// OneTimeToken represents a token for secure operations
type OneTimeToken struct {
	mu sync.RWMutex

	ID        string     `json:"id"`
	Token     string     `json:"token"`
	Type      TokenType  `json:"type"`
	State     TokenState `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	MaxUses   int        `json:"max_uses"`
	UseCount  int        `json:"use_count"`

	// Context data
	Payload  string            `json:"payload,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// Usage tracking
	UsedBy []TokenUsage `json:"used_by,omitempty"`

	// Signature
	Signature string `json:"signature"`
}

// TokenUsage tracks token usage
type TokenUsage struct {
	UsedAt    time.Time `json:"used_at"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
}

// QRManager handles QR code generation and token management
type QRManager struct {
	mu         sync.RWMutex
	tokens     map[string]*OneTimeToken
	signingKey []byte
	config     QRConfig

	serverURL  string
	bridgeURL  string
	serverName string
	wsURL      string

	tlsMode          string
	tlsFingerprint   string
	tlsTrustHint     string
	tlsCertExpiresAt int64
}

// QRConfig configures QR code behavior
type QRConfig struct {
	DefaultTokenExpiration time.Duration
	MaxActiveTokens        int
	QRSize                 int
	QRRecoveryLevel        qrcode.RecoveryLevel
}

// DefaultQRConfig returns sensible defaults
func DefaultQRConfig() QRConfig {
	return QRConfig{
		DefaultTokenExpiration: 10 * time.Minute,
		MaxActiveTokens:        10,
		QRSize:                 256,
		QRRecoveryLevel:        qrcode.Medium,
	}
}

// NewQRManager creates a new QR manager
func NewQRManager(signingKey []byte, config QRConfig, serverURL, bridgeURL, serverName string) *QRManager {
	if len(signingKey) == 0 {
		signingKey = securerandom.MustBytes(32)
	}

	return &QRManager{
		tokens:     make(map[string]*OneTimeToken),
		signingKey: signingKey,
		config:     config,
		serverURL:  serverURL,
		bridgeURL:  bridgeURL,
		serverName: serverName,
	}
}

func (m *QRManager) SetTLSInfo(mode, fingerprint, trustHint string, certExpiresAt int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tlsMode = mode
	m.tlsFingerprint = fingerprint
	m.tlsTrustHint = trustHint
	m.tlsCertExpiresAt = certExpiresAt
}

func (m *QRManager) SetWsURL(wsURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wsURL = wsURL
}

// GenerateSetupQR generates a QR code for initial setup
func (m *QRManager) GenerateSetupQR() (*QRResult, error) {
	token, err := m.createToken(TokenTypeSetup, "", m.config.DefaultTokenExpiration, 1, nil)
	if err != nil {
		return nil, err
	}

	// Create setup URL
	setupURL := fmt.Sprintf("https://armorclaw.app/setup?token=%s&server=%s",
		url.QueryEscape(token.Token),
		url.QueryEscape(m.serverURL),
	)

	// Generate QR code
	qrBytes, err := qrcode.Encode(setupURL, m.config.QRRecoveryLevel, m.config.QRSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	return &QRResult{
		Token:     token,
		URL:       setupURL,
		DeepLink:  fmt.Sprintf("armorclaw://setup?token=%s&server=%s", token.Token, m.serverURL),
		QRImage:   qrBytes,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// GenerateBondingQR generates a QR code for admin bonding
func (m *QRManager) GenerateBondingQR(challenge string) (*QRResult, error) {
	payload := base64.URLEncoding.EncodeToString([]byte(challenge))
	token, err := m.createToken(TokenTypeBonding, payload, m.config.DefaultTokenExpiration, 1, nil)
	if err != nil {
		return nil, err
	}

	// Create bonding URL
	bondingURL := fmt.Sprintf("https://armorclaw.app/bond?token=%s&challenge=%s&server=%s",
		url.QueryEscape(token.Token),
		url.QueryEscape(challenge),
		url.QueryEscape(m.serverURL),
	)

	qrBytes, err := qrcode.Encode(bondingURL, m.config.QRRecoveryLevel, m.config.QRSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	return &QRResult{
		Token:     token,
		URL:       bondingURL,
		DeepLink:  fmt.Sprintf("armorclaw://bond?token=%s&challenge=%s", token.Token, challenge),
		QRImage:   qrBytes,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// GenerateAPISecretQR generates a QR code for API key injection
func (m *QRManager) GenerateAPISecretQR(provider string) (*QRResult, error) {
	metadata := map[string]string{"provider": provider}
	token, err := m.createToken(TokenTypeAPISecret, provider, m.config.DefaultTokenExpiration, 1, metadata)
	if err != nil {
		return nil, err
	}

	secretURL := fmt.Sprintf("https://armorclaw.app/secret?token=%s&provider=%s",
		url.QueryEscape(token.Token),
		url.QueryEscape(provider),
	)

	qrBytes, err := qrcode.Encode(secretURL, m.config.QRRecoveryLevel, m.config.QRSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	return &QRResult{
		Token:     token,
		URL:       secretURL,
		DeepLink:  fmt.Sprintf("armorclaw://secret?token=%s&provider=%s", token.Token, provider),
		QRImage:   qrBytes,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// GenerateInviteQR generates a QR code for an invitation
func (m *QRManager) GenerateInviteQR(inviteCode, role string) (*QRResult, error) {
	metadata := map[string]string{"role": role}
	token, err := m.createToken(TokenTypeInvite, inviteCode, 24*time.Hour, 1, metadata)
	if err != nil {
		return nil, err
	}

	inviteURL := fmt.Sprintf("https://armorclaw.app/invite/%s?token=%s",
		inviteCode,
		url.QueryEscape(token.Token),
	)

	qrBytes, err := qrcode.Encode(inviteURL, m.config.QRRecoveryLevel, m.config.QRSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	return &QRResult{
		Token:     token,
		URL:       inviteURL,
		DeepLink:  fmt.Sprintf("armorclaw://invite?code=%s", inviteCode),
		QRImage:   qrBytes,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// GenerateVerificationQR generates a QR code for device verification
func (m *QRManager) GenerateVerificationQR(deviceID string) (*QRResult, error) {
	metadata := map[string]string{"device_id": deviceID}
	token, err := m.createToken(TokenTypeVerification, deviceID, 5*time.Minute, 1, metadata)
	if err != nil {
		return nil, err
	}

	verifyURL := fmt.Sprintf("https://armorclaw.app/verify?token=%s&device=%s",
		url.QueryEscape(token.Token),
		url.QueryEscape(deviceID),
	)

	qrBytes, err := qrcode.Encode(verifyURL, m.config.QRRecoveryLevel, m.config.QRSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	return &QRResult{
		Token:     token,
		URL:       verifyURL,
		DeepLink:  fmt.Sprintf("armorclaw://verify?token=%s&device=%s", token.Token, deviceID),
		QRImage:   qrBytes,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// ConfigPayload contains server configuration for client apps
type ConfigPayload struct {
	Version               int    `json:"version"`
	MatrixHomeserver      string `json:"matrix_homeserver"`
	RpcURL                string `json:"rpc_url"`
	WsURL                 string `json:"ws_url,omitempty"`
	PushGateway           string `json:"push_gateway"`
	ServerName            string `json:"server_name"`
	Region                string `json:"region,omitempty"`
	TLSMode               string `json:"tls_mode,omitempty"`
	TLSFingerprintSHA256  string `json:"tls_fingerprint_sha256,omitempty"`
	TLSTrustHint          string `json:"tls_trust_hint,omitempty"`
	CertExpiresAt         int64  `json:"cert_expires_at,omitempty"`
	ExpiresAt             int64  `json:"expires_at"`
	Signature             string `json:"signature"`
}

// GenerateConfigQR generates a QR code with server configuration for ArmorTerminal/ArmorChat
// This allows users to scan a QR code after app launch to auto-configure all server URLs.
func (m *QRManager) GenerateConfigQR(expiration time.Duration) (*ConfigQRResult, error) {
	if expiration == 0 {
		expiration = 24 * time.Hour // Default 24 hours for config tokens
	}

	// Create config payload
	config := &ConfigPayload{
		Version:          1,
		MatrixHomeserver: m.serverURL,
		RpcURL:           m.bridgeURL + "/api",
		PushGateway:      m.bridgeURL + "/_matrix/push/v1/notify",
		ServerName:       m.serverName,
		ExpiresAt:        time.Now().Add(expiration).Unix(),
	}

	if m.wsURL != "" {
		config.WsURL = m.wsURL
	}

	if os.Getenv("ARMORCLAW_QR_VERSION") == "2" {
		config.Version = 2
		config.TLSMode = m.tlsMode
		config.TLSFingerprintSHA256 = m.tlsFingerprint
		config.TLSTrustHint = m.tlsTrustHint
		config.CertExpiresAt = m.tlsCertExpiresAt
	}

	// Sign the config
	config.Signature = m.signConfig(config)

	// Encode config as JSON, then base64
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	configB64 := base64.URLEncoding.EncodeToString(configJSON)

	// Create deep link URL
	deepLink := fmt.Sprintf("armorclaw://config?d=%s", configB64)

	// Create web URL (for browsers)
	webURL := fmt.Sprintf("https://armorclaw.app/config?d=%s", configB64)

	// Generate QR code
	qrBytes, err := qrcode.Encode(deepLink, m.config.QRRecoveryLevel, m.config.QRSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR: %w", err)
	}

	// Create a token for tracking
	token, err := m.createToken(TokenTypeConfig, string(configJSON), expiration, 10, map[string]string{
		"server_name": m.serverName,
	})
	if err != nil {
		return nil, err
	}

	return &ConfigQRResult{
		Token:     token,
		Config:    config,
		URL:       webURL,
		DeepLink:  deepLink,
		QRImage:   qrBytes,
		ExpiresAt: token.ExpiresAt,
	}, nil
}

// GenerateConfigURL generates a signed configuration URL (no QR image)
func (m *QRManager) GenerateConfigURL(expiration time.Duration) (string, *ConfigPayload, error) {
	if expiration == 0 {
		expiration = 24 * time.Hour
	}

	config := &ConfigPayload{
		Version:          1,
		MatrixHomeserver: m.serverURL,
		RpcURL:           m.bridgeURL + "/api",
		PushGateway:      m.bridgeURL + "/_matrix/push/v1/notify",
		ServerName:       m.serverName,
		ExpiresAt:        time.Now().Add(expiration).Unix(),
	}

	if m.wsURL != "" {
		config.WsURL = m.wsURL
	}

	if os.Getenv("ARMORCLAW_QR_VERSION") == "2" {
		config.Version = 2
		config.TLSMode = m.tlsMode
		config.TLSFingerprintSHA256 = m.tlsFingerprint
		config.TLSTrustHint = m.tlsTrustHint
		config.CertExpiresAt = m.tlsCertExpiresAt
	}

	config.Signature = m.signConfig(config)

	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	configB64 := base64.URLEncoding.EncodeToString(configJSON)

	url := fmt.Sprintf("armorclaw://config?d=%s", configB64)
	return url, config, nil
}

// ValidateConfig validates a signed configuration payload
func (m *QRManager) ValidateConfig(config *ConfigPayload) error {
	// Check expiration
	if time.Now().Unix() > config.ExpiresAt {
		return errors.New("config expired")
	}

	// Verify signature
	expectedSig := m.signConfig(config)
	if !hmac.Equal([]byte(config.Signature), []byte(expectedSig)) {
		return errors.New("invalid config signature")
	}

	return nil
}

// ParseConfigURL parses a config URL and returns the payload
func ParseConfigURL(configURL string) (*ConfigPayload, error) {
	// Parse armorclaw://config?d=...
	u, err := url.Parse(configURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "armorclaw" || u.Host != "config" {
		return nil, errors.New("not a config URL")
	}

	data := u.Query().Get("d")
	if data == "" {
		return nil, errors.New("missing config data")
	}

	// Decode base64
	configJSON, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}

	// Parse JSON
	var config ConfigPayload
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &config, nil
}

// signConfig generates a signature for a config payload
func (m *QRManager) signConfig(config *ConfigPayload) string {
	var data string
	if config.Version == 1 {
		data = fmt.Sprintf("%d:%s:%s:%s:%s:%s:%d",
			config.Version,
			config.MatrixHomeserver,
			config.RpcURL,
			config.WsURL,
			config.PushGateway,
			config.ServerName,
			config.ExpiresAt,
		)
	} else {
		data = fmt.Sprintf("%d:%s:%s:%s:%s:%s:%s:%s:%s:%d:%d",
			config.Version,
			config.MatrixHomeserver,
			config.RpcURL,
			config.WsURL,
			config.PushGateway,
			config.ServerName,
			config.TLSMode,
			config.TLSFingerprintSHA256,
			config.TLSTrustHint,
			config.CertExpiresAt,
			config.ExpiresAt,
		)
	}

	h := hmac.New(sha256.New, m.signingKey)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ConfigQRResult contains the result of config QR generation
type ConfigQRResult struct {
	Token     *OneTimeToken  `json:"token"`
	Config    *ConfigPayload `json:"config"`
	URL       string         `json:"url"`       // Web URL
	DeepLink  string         `json:"deep_link"` // armorclaw:// deep link
	QRImage   []byte         `json:"qr_image"`  // PNG QR code
	ExpiresAt time.Time      `json:"expires_at"`
}

// QRResult contains the result of QR code generation
type QRResult struct {
	Token     *OneTimeToken `json:"token"`
	URL       string        `json:"url"`
	DeepLink  string        `json:"deep_link"`
	QRImage   []byte        `json:"qr_image"`
	ExpiresAt time.Time     `json:"expires_at"`
}

// ToTerminal returns an ASCII QR code for terminal display
// using the skip2/go-qrcode library already in use
func (r *QRResult) ToTerminal() (string, error) {
	if strings.TrimSpace(r.DeepLink) == "" {
		return "", fmt.Errorf("deep link is empty")
	}
	qrCode, err := qrcode.New(r.DeepLink, qrcode.Medium)
	if err != nil {
		return "", err
	}
	return qrCode.ToSmallString(false), nil
}

// createToken creates a new one-time token
func (m *QRManager) createToken(tokenType TokenType, payload string, expiration time.Duration, maxUses int, metadata map[string]string) (*OneTimeToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check active token limit
	activeCount := 0
	for _, t := range m.tokens {
		if t.State == TokenStateActive {
			activeCount++
		}
	}
	if activeCount >= m.config.MaxActiveTokens {
		return nil, errors.New("maximum active tokens reached")
	}

	// Generate token
	tokenID := "tok_" + securerandom.MustID(16)
	tokenStr := base64.URLEncoding.EncodeToString(securerandom.MustBytes(24))

	now := time.Now()
	token := &OneTimeToken{
		ID:        tokenID,
		Token:     tokenStr,
		Type:      tokenType,
		State:     TokenStateActive,
		CreatedAt: now,
		ExpiresAt: now.Add(expiration),
		MaxUses:   maxUses,
		Payload:   payload,
		Metadata:  metadata,
	}

	// Sign token
	token.Signature = m.signToken(token)

	m.tokens[tokenID] = token
	return token, nil
}

// ValidateToken validates a token string
func (m *QRManager) ValidateToken(tokenStr string) (*OneTimeToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var token *OneTimeToken
	for _, t := range m.tokens {
		if t.Token == tokenStr {
			token = t
			break
		}
	}

	if token == nil {
		return nil, errors.New("invalid token")
	}

	token.mu.RLock()
	defer token.mu.RUnlock()

	// Verify signature
	expectedSig := m.signToken(token)
	if !hmac.Equal([]byte(token.Signature), []byte(expectedSig)) {
		return nil, errors.New("invalid token signature")
	}

	// Check state
	switch token.State {
	case TokenStateUsed:
		return nil, errors.New("token already used")
	case TokenStateExpired:
		return nil, errors.New("token expired")
	case TokenStateRevoked:
		return nil, errors.New("token revoked")
	}

	// Check expiration
	if time.Now().After(token.ExpiresAt) {
		return nil, errors.New("token expired")
	}

	return token, nil
}

// UseToken marks a token as used and returns its payload
func (m *QRManager) UseToken(tokenStr, userAgent, ipAddress string) (*OneTimeToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var token *OneTimeToken
	for _, t := range m.tokens {
		if t.Token == tokenStr {
			token = t
			break
		}
	}

	if token == nil {
		return nil, errors.New("token not found")
	}

	token.mu.Lock()
	defer token.mu.Unlock()

	// Validate
	switch token.State {
	case TokenStateUsed:
		return nil, errors.New("token already used")
	case TokenStateRevoked:
		return nil, errors.New("token revoked")
	}

	if time.Now().After(token.ExpiresAt) {
		token.State = TokenStateExpired
		return nil, errors.New("token expired")
	}

	// Record usage
	token.UseCount++
	token.UsedBy = append(token.UsedBy, TokenUsage{
		UsedAt:    time.Now(),
		UserAgent: userAgent,
		IPAddress: ipAddress,
	})

	// Check if exhausted
	if token.MaxUses > 0 && token.UseCount >= token.MaxUses {
		token.State = TokenStateUsed
	}

	return token, nil
}

// RevokeToken revokes a token
func (m *QRManager) RevokeToken(tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, exists := m.tokens[tokenID]
	if !exists {
		return errors.New("token not found")
	}

	token.mu.Lock()
	defer token.mu.Unlock()

	token.State = TokenStateRevoked
	return nil
}

// CleanupExpired removes expired tokens
func (m *QRManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for id, token := range m.tokens {
		token.mu.Lock()
		if token.State == TokenStateActive && now.After(token.ExpiresAt) {
			token.State = TokenStateExpired
			count++
		}
		token.mu.Unlock()

		// Remove old non-active tokens
		if token.State != TokenStateActive {
			if now.Sub(token.CreatedAt) > 24*time.Hour {
				delete(m.tokens, id)
			}
		}
	}

	return count
}

// signToken generates a signature for a token
func (m *QRManager) signToken(token *OneTimeToken) string {
	data := fmt.Sprintf("%s:%s:%s:%s:%d",
		token.ID,
		token.Token,
		token.Type,
		token.CreatedAt.Format(time.RFC3339),
		token.MaxUses,
	)

	h := hmac.New(sha256.New, m.signingKey)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ToJSON returns the token as JSON
func (t *OneTimeToken) ToJSON() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return json.MarshalIndent(t, "", "  ")
}

// Summary returns a summary of the token
func (t *OneTimeToken) Summary() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"id":         t.ID,
		"type":       string(t.Type),
		"state":      string(t.State),
		"created_at": t.CreatedAt,
		"expires_at": t.ExpiresAt,
		"use_count":  t.UseCount,
	}
}

// toWSS converts an https:// URL to wss:// (or http:// to ws://)
func toWSS(u string) string {
	if len(u) >= 8 && u[:8] == "https://" {
		return "wss://" + u[8:]
	}
	if len(u) >= 7 && u[:7] == "http://" {
		return "ws://" + u[7:]
	}
	return u
}

// QRToImage converts QR bytes to an image.Image
func QRToImage(qrBytes []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(qrBytes))
}

// Stats returns token statistics
func (m *QRManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]int{
		"total":   len(m.tokens),
		"active":  0,
		"used":    0,
		"expired": 0,
		"revoked": 0,
	}

	byType := make(map[TokenType]int)

	for _, token := range m.tokens {
		token.mu.RLock()
		switch token.State {
		case TokenStateActive:
			stats["active"]++
		case TokenStateUsed:
			stats["used"]++
		case TokenStateExpired:
			stats["expired"]++
		case TokenStateRevoked:
			stats["revoked"]++
		}
		byType[token.Type]++
		token.mu.RUnlock()
	}

	return map[string]interface{}{
		"state":   stats,
		"by_type": byType,
	}
}
