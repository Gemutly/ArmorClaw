// Package audit provides audit logging helpers for critical operations
package audit

import (
	"context"
	"sync"
)

// CriticalOperationLogger provides audit logging for critical operations
type CriticalOperationLogger struct {
	auditLog *TamperEvidentLog
	mu       sync.RWMutex
}

// NewCriticalOperationLogger creates a new critical operation logger
func NewCriticalOperationLogger(auditLog *TamperEvidentLog) *CriticalOperationLogger {
	return &CriticalOperationLogger{
		auditLog: auditLog,
	}
}

// SetAuditLog updates the audit log
func (l *CriticalOperationLogger) SetAuditLog(auditLog *TamperEvidentLog) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.auditLog = auditLog
}

// LogContainerStart logs a container start operation
func (l *CriticalOperationLogger) LogContainerStart(ctx context.Context, containerID, image, keyID, sessionID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type:      "system",
		ID:        "bridge",
		IPAddress: "",
	}
	resource := Resource{
		Type: "container",
		ID:   containerID,
		Name: image,
	}
	details := map[string]interface{}{
		"image":      image,
		"key_id":     keyID,
		"session_id": sessionID,
		"action":     "start",
	}
	compliance := ComplianceFlags{
		Category:      "container_lifecycle",
		Severity:      "medium",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("container_start", actor, "start", resource, details, compliance)
	return err
}

// LogContainerStop logs a container stop operation
func (l *CriticalOperationLogger) LogContainerStop(ctx context.Context, containerID, reason string, exitCode int) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "bridge",
	}
	resource := Resource{
		Type: "container",
		ID:   containerID,
	}
	details := map[string]interface{}{
		"reason":    reason,
		"exit_code": exitCode,
		"action":    "stop",
	}
	compliance := ComplianceFlags{
		Category:      "container_lifecycle",
		Severity:      "medium",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("container_stop", actor, "stop", resource, details, compliance)
	return err
}

// LogContainerError logs a container error
func (l *CriticalOperationLogger) LogContainerError(ctx context.Context, containerID, errorMsg string, errorCode string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "bridge",
	}
	resource := Resource{
		Type: "container",
		ID:   containerID,
	}
	details := map[string]interface{}{
		"error":      errorMsg,
		"error_code": errorCode,
		"action":     "error",
	}
	compliance := ComplianceFlags{
		Category:      "container_lifecycle",
		Severity:      "high",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("container_error", actor, "error", resource, details, compliance)
	return err
}

// LogKeyAccess logs a key access operation
func (l *CriticalOperationLogger) LogKeyAccess(ctx context.Context, keyID, userID, operation string, success bool) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "api_key",
		ID:   keyID,
	}
	severity := "medium"
	if !success {
		severity = "high"
	}
	details := map[string]interface{}{
		"operation": operation,
		"success":   success,
	}
	compliance := ComplianceFlags{
		Category:      "key_access",
		Severity:      severity,
		AuditRequired: true,
	}

	eventType := "key_access"
	if !success {
		eventType = "key_access_denied"
	}

	_, err := auditLog.LogEntry(eventType, actor, operation, resource, details, compliance)
	return err
}

// LogKeyCreated logs a key creation operation
func (l *CriticalOperationLogger) LogKeyCreated(ctx context.Context, keyID, provider, userID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "api_key",
		ID:   keyID,
	}
	details := map[string]interface{}{
		"provider": provider,
		"action":   "create",
	}
	compliance := ComplianceFlags{
		Category:      "key_management",
		Severity:      "high",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("key_created", actor, "create", resource, details, compliance)
	return err
}

// LogKeyDeleted logs a key deletion operation
func (l *CriticalOperationLogger) LogKeyDeleted(ctx context.Context, keyID, userID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "api_key",
		ID:   keyID,
	}
	details := map[string]interface{}{
		"action": "delete",
	}
	compliance := ComplianceFlags{
		Category:      "key_management",
		Severity:      "high",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("key_deleted", actor, "delete", resource, details, compliance)
	return err
}

// LogSecretInjection logs a secret injection operation
func (l *CriticalOperationLogger) LogSecretInjection(ctx context.Context, containerID, keyID string, success bool) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "bridge",
	}
	resource := Resource{
		Type: "container",
		ID:   containerID,
	}
	severity := "low"
	if !success {
		severity = "high"
	}
	details := map[string]interface{}{
		"key_id":  keyID,
		"success": success,
		"action":  "inject",
	}
	compliance := ComplianceFlags{
		Category:      "secret_management",
		Severity:      severity,
		AuditRequired: true,
		PHIInvolved:   false, // Secrets are not PHI
	}

	eventType := "secret_injected"
	if !success {
		eventType = "secret_injection_failed"
	}

	_, err := auditLog.LogEntry(eventType, actor, "inject", resource, details, compliance)
	return err
}

// LogSecretCleanup logs a secret cleanup operation
func (l *CriticalOperationLogger) LogSecretCleanup(ctx context.Context, containerID string, success bool) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "bridge",
	}
	resource := Resource{
		Type: "container",
		ID:   containerID,
	}
	details := map[string]interface{}{
		"success": success,
		"action":  "cleanup",
	}
	compliance := ComplianceFlags{
		Category:      "secret_management",
		Severity:      "low",
		AuditRequired: false,
	}

	_, err := auditLog.LogEntry("secret_cleaned", actor, "cleanup", resource, details, compliance)
	return err
}

// LogConfigurationChange logs a configuration change
func (l *CriticalOperationLogger) LogConfigurationChange(ctx context.Context, userID, section, key string, oldValue, newValue interface{}) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "configuration",
		ID:   section,
	}
	details := map[string]interface{}{
		"key":       key,
		"old_value": oldValue,
		"new_value": newValue,
		"action":    "change",
	}
	compliance := ComplianceFlags{
		Category:      "configuration",
		Severity:      "medium",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("config_change", actor, "change", resource, details, compliance)
	return err
}

// LogAuthenticationEvent logs an authentication event
func (l *CriticalOperationLogger) LogAuthenticationEvent(ctx context.Context, userID, method string, success bool, ipAddress string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type:      "user",
		ID:        userID,
		IPAddress: ipAddress,
	}
	resource := Resource{
		Type: "session",
		ID:   userID,
	}
	severity := "low"
	if !success {
		severity = "high"
	}
	details := map[string]interface{}{
		"method":  method,
		"success": success,
	}
	compliance := ComplianceFlags{
		Category:      "authentication",
		Severity:      severity,
		AuditRequired: true,
	}

	eventType := "auth_success"
	if !success {
		eventType = "auth_failure"
	}

	_, err := auditLog.LogEntry(eventType, actor, "authenticate", resource, details, compliance)
	return err
}

// LogSecurityEvent logs a security-related event
func (l *CriticalOperationLogger) LogSecurityEvent(ctx context.Context, eventType, severity string, details map[string]interface{}) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "bridge",
	}
	resource := Resource{
		Type: "security",
		ID:   eventType,
	}
	compliance := ComplianceFlags{
		Category:      "security",
		Severity:      severity,
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry(eventType, actor, "security_event", resource, details, compliance)
	return err
}

// LogBudgetEvent logs a budget-related event
func (l *CriticalOperationLogger) LogBudgetEvent(ctx context.Context, sessionID string, tokensUsed, tokensLimit int, exceeded bool) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "budget_tracker",
	}
	resource := Resource{
		Type: "session",
		ID:   sessionID,
	}
	severity := "low"
	if exceeded {
		severity = "high"
	}
	details := map[string]interface{}{
		"tokens_used":  tokensUsed,
		"tokens_limit": tokensLimit,
		"exceeded":     exceeded,
	}
	compliance := ComplianceFlags{
		Category:      "budget",
		Severity:      severity,
		AuditRequired: exceeded,
	}

	eventType := "budget_warning"
	if exceeded {
		eventType = "budget_exceeded"
	}

	_, err := auditLog.LogEntry(eventType, actor, "budget_check", resource, details, compliance)
	return err
}

// LogPHIAccess logs access to PHI (Protected Health Information)
func (l *CriticalOperationLogger) LogPHIAccess(ctx context.Context, userID, resourceType, resourceID, action string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: resourceType,
		ID:   resourceID,
	}
	details := map[string]interface{}{
		"action": action,
	}
	compliance := ComplianceFlags{
		Category:      "phi_access",
		Severity:      "high",
		AuditRequired: true,
		PHIInvolved:   true,
	}

	_, err := auditLog.LogEntry("phi_access", actor, action, resource, details, compliance)
	return err
}

// LogProfileStored logs when a user profile is stored
// CRITICAL: Never log actual PII values - only profile ID and type
func (l *CriticalOperationLogger) LogProfileStored(ctx context.Context, profileID, profileType string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "keystore",
	}
	resource := Resource{
		Type: "pii_profile",
		ID:   profileID,
	}
	// Only log metadata, never PII values
	details := map[string]interface{}{
		"profile_type": profileType,
		"action":       "store",
	}
	compliance := ComplianceFlags{
		Category:      "pii_management",
		Severity:      "high",
		AuditRequired: true,
		PHIInvolved:   true,
	}

	_, err := auditLog.LogEntry("pii_profile_stored", actor, "store", resource, details, compliance)
	return err
}

// LogProfileAccess logs when a user profile is accessed
// CRITICAL: Never log actual PII values - only profile ID and operation
func (l *CriticalOperationLogger) LogProfileAccess(ctx context.Context, profileID, operation string, success bool) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "keystore",
	}
	resource := Resource{
		Type: "pii_profile",
		ID:   profileID,
	}
	severity := "medium"
	if !success {
		severity = "high"
	}
	// Only log metadata, never PII values
	details := map[string]interface{}{
		"operation": operation,
		"success":   success,
	}
	compliance := ComplianceFlags{
		Category:      "pii_access",
		Severity:      severity,
		AuditRequired: true,
		PHIInvolved:   true,
	}

	eventType := "pii_profile_access"
	if !success {
		eventType = "pii_profile_access_denied"
	}

	_, err := auditLog.LogEntry(eventType, actor, operation, resource, details, compliance)
	return err
}

// LogProfileDeleted logs when a user profile is deleted
func (l *CriticalOperationLogger) LogProfileDeleted(ctx context.Context, profileID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "keystore",
	}
	resource := Resource{
		Type: "pii_profile",
		ID:   profileID,
	}
	details := map[string]interface{}{
		"action": "delete",
	}
	compliance := ComplianceFlags{
		Category:      "pii_management",
		Severity:      "high",
		AuditRequired: true,
		PHIInvolved:   false, // PII is deleted, no longer involved
	}

	_, err := auditLog.LogEntry("pii_profile_deleted", actor, "delete", resource, details, compliance)
	return err
}

// LogPIIAccessRequest logs when a skill requests access to PII fields
// CRITICAL: Never log actual PII values - only field names and hashes
func (l *CriticalOperationLogger) LogPIIAccessRequest(ctx context.Context, requestID, skillID, profileID string, requestedFields []string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "skill",
		ID:   skillID,
	}
	resource := Resource{
		Type: "pii_profile",
		ID:   profileID,
	}
	// Only log field names, never values
	details := map[string]interface{}{
		"request_id":       requestID,
		"requested_fields": requestedFields,
		"action":           "request_access",
	}
	compliance := ComplianceFlags{
		Category:      "pii_access",
		Severity:      "high",
		AuditRequired: true,
		PHIInvolved:   true,
	}

	_, err := auditLog.LogEntry("pii_access_request", actor, "request", resource, details, compliance)
	return err
}

// LogPIIAccessGranted logs when PII access is granted
// CRITICAL: Never log actual PII values - only field names and approver
func (l *CriticalOperationLogger) LogPIIAccessGranted(ctx context.Context, requestID, skillID, userID string, approvedFields []string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "pii_request",
		ID:   requestID,
	}
	// Only log field names, never values
	details := map[string]interface{}{
		"skill_id":        skillID,
		"approved_fields": approvedFields,
		"action":          "approve",
	}
	compliance := ComplianceFlags{
		Category:      "pii_access",
		Severity:      "high",
		AuditRequired: true,
		PHIInvolved:   true,
	}

	_, err := auditLog.LogEntry("pii_access_granted", actor, "approve", resource, details, compliance)
	return err
}

// LogPIIAccessRejected logs when PII access is rejected
func (l *CriticalOperationLogger) LogPIIAccessRejected(ctx context.Context, requestID, skillID, userID, reason string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "pii_request",
		ID:   requestID,
	}
	details := map[string]interface{}{
		"skill_id": skillID,
		"reason":   reason,
		"action":   "reject",
	}
	compliance := ComplianceFlags{
		Category:      "pii_access",
		Severity:      "medium",
		AuditRequired: true,
		PHIInvolved:   false,
	}

	_, err := auditLog.LogEntry("pii_access_rejected", actor, "reject", resource, details, compliance)
	return err
}

// LogPIIAccessExpired logs when a PII access request expires
func (l *CriticalOperationLogger) LogPIIAccessExpired(ctx context.Context, requestID, skillID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "hitl_manager",
	}
	resource := Resource{
		Type: "pii_request",
		ID:   requestID,
	}
	details := map[string]interface{}{
		"skill_id": skillID,
		"action":   "expire",
	}
	compliance := ComplianceFlags{
		Category:      "pii_access",
		Severity:      "medium",
		AuditRequired: true,
		PHIInvolved:   false,
	}

	_, err := auditLog.LogEntry("pii_access_expired", actor, "expire", resource, details, compliance)
	return err
}

// LogPIIInjected logs when PII is injected into a container
// CRITICAL: Never log actual PII values - only field names and container ID
func (l *CriticalOperationLogger) LogPIIInjected(ctx context.Context, containerID, skillID string, fieldsInjected []string, method string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "skill",
		ID:   skillID,
	}
	resource := Resource{
		Type: "container",
		ID:   containerID,
	}
	// Only log field names, never values
	details := map[string]interface{}{
		"fields_injected": fieldsInjected,
		"injection_method": method,
		"action":          "inject",
	}
	compliance := ComplianceFlags{
		Category:      "pii_injection",
		Severity:      "high",
		AuditRequired: true,
		PHIInvolved:   true,
	}

	_, err := auditLog.LogEntry("pii_injected", actor, "inject", resource, details, compliance)
	return err
}


// SensitiveFieldDenylist contains field names that must NEVER appear in audit logs.
// Covers: passwords, verifiers, vault keys, recovery phrases, wrapped keys,
// Ed25519 challenge nonces, and PII values.
var SensitiveFieldDenylist = map[string]bool{
	"password":        true,
	"passwd":          true,
	"pin":             true,
	"passphrase":      true,
	"verifier":        true,
	"verifier_hash":   true,
	"vault_key":       true,
	"vault_keys":      true,
	"master_key":      true,
	"recovery_phrase": true,
	"seed_phrase":     true,
	"mnemonic":        true,
	"wrapped_key":     true,
	"wrapped_keys":    true,
	"challenge_nonce": true,
	"nonce":           true,
	"secret":          true,
	"token":           true,
	"access_token":    true,
	"refresh_token":   true,
	"api_key":         true,
	"private_key":     true,
	"ssn":             true,
	"credit_card":     true,
	"card_number":     true,
	"cvv":             true,
}

// SanitizeDetails removes sensitive fields from a details map.
// Returns a new map with denylisted fields stripped.
func SanitizeDetails(details map[string]interface{}) map[string]interface{} {
	if details == nil {
		return nil
	}
	safe := make(map[string]interface{}, len(details))
	for k, v := range details {
		if SensitiveFieldDenylist[k] {
			continue
		}
		safe[k] = v
	}
	return safe
}

// LogKeystoreUnseal logs a keystore unseal event.
// Logs identity + success/failure. NEVER logs the password.
func (l *CriticalOperationLogger) LogKeystoreUnseal(ctx context.Context, identity string, success bool) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   identity,
	}
	resource := Resource{
		Type: "keystore",
		ID:   identity,
	}
	severity := "low"
	if !success {
		severity = "high"
	}
	details := map[string]interface{}{
		"success": success,
		"action":  "unseal",
	}
	compliance := ComplianceFlags{
		Category:      "keystore_lifecycle",
		Severity:      severity,
		AuditRequired: true,
	}

	eventType := "keystore.unseal"
	if !success {
		eventType = "keystore.unseal_failed"
	}

	_, err := auditLog.LogEntry(eventType, actor, "unseal", resource, details, compliance)
	return err
}

// LogKeystoreSeal logs a keystore seal event.
// Logs identity + reason (manual vs auto-seal timer).
func (l *CriticalOperationLogger) LogKeystoreSeal(ctx context.Context, identity, reason string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   identity,
	}
	resource := Resource{
		Type: "keystore",
		ID:   identity,
	}
	details := map[string]interface{}{
		"reason": reason,
		"action": "seal",
	}
	compliance := ComplianceFlags{
		Category:      "keystore_lifecycle",
		Severity:      "medium",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("keystore.seal", actor, "seal", resource, details, compliance)
	return err
}

// LogKeystoreExtendSession logs a keystore session extension event.
func (l *CriticalOperationLogger) LogKeystoreExtendSession(ctx context.Context, identity string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   identity,
	}
	resource := Resource{
		Type: "keystore",
		ID:   identity,
	}
	details := map[string]interface{}{
		"action": "extend_session",
	}
	compliance := ComplianceFlags{
		Category:      "keystore_lifecycle",
		Severity:      "low",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("keystore.extend_session", actor, "extend_session", resource, details, compliance)
	return err
}

// LogBackupCreate logs a backup creation event.
// Logs user ID + backup ID. NEVER logs recovery phrase or encrypted key.
func (l *CriticalOperationLogger) LogBackupCreate(ctx context.Context, userID, backupID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "backup",
		ID:   backupID,
	}
	details := map[string]interface{}{
		"backup_id": backupID,
		"action":    "create",
	}
	compliance := ComplianceFlags{
		Category:      "backup_management",
		Severity:      "medium",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("backup.create", actor, "create", resource, details, compliance)
	return err
}

// LogBackupDelete logs a backup deletion event.
func (l *CriticalOperationLogger) LogBackupDelete(ctx context.Context, userID, backupID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "user",
		ID:   userID,
	}
	resource := Resource{
		Type: "backup",
		ID:   backupID,
	}
	details := map[string]interface{}{
		"backup_id": backupID,
		"action":    "delete",
	}
	compliance := ComplianceFlags{
		Category:      "backup_management",
		Severity:      "high",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("backup.delete", actor, "delete", resource, details, compliance)
	return err
}

// LogVoiceStartSession logs a voice session start event.
func (l *CriticalOperationLogger) LogVoiceStartSession(ctx context.Context, sessionID, provider string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "voice_service",
	}
	resource := Resource{
		Type: "voice_session",
		ID:   sessionID,
	}
	details := map[string]interface{}{
		"session_id": sessionID,
		"provider":   provider,
		"action":     "start",
	}
	compliance := ComplianceFlags{
		Category:      "voice_session",
		Severity:      "low",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("voice.start_session", actor, "start", resource, details, compliance)
	return err
}

// LogVoiceStopSession logs a voice session stop event.
func (l *CriticalOperationLogger) LogVoiceStopSession(ctx context.Context, sessionID string) error {
	l.mu.RLock()
	auditLog := l.auditLog
	l.mu.RUnlock()

	if auditLog == nil {
		return nil
	}

	actor := Actor{
		Type: "system",
		ID:   "voice_service",
	}
	resource := Resource{
		Type: "voice_session",
		ID:   sessionID,
	}
	details := map[string]interface{}{
		"session_id": sessionID,
		"action":     "stop",
	}
	compliance := ComplianceFlags{
		Category:      "voice_session",
		Severity:      "low",
		AuditRequired: true,
	}

	_, err := auditLog.LogEntry("voice.stop_session", actor, "stop", resource, details, compliance)
	return err
}

// Global audit logger instance
var globalAuditLogger *CriticalOperationLogger
var globalAuditMu sync.RWMutex

// SetGlobalAuditLogger sets the global audit logger
func SetGlobalAuditLogger(logger *CriticalOperationLogger) {
	globalAuditMu.Lock()
	defer globalAuditMu.Unlock()
	globalAuditLogger = logger
}

// GetGlobalAuditLogger gets the global audit logger
func GetGlobalAuditLogger() *CriticalOperationLogger {
	globalAuditMu.RLock()
	defer globalAuditMu.RUnlock()
	return globalAuditLogger
}
