package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// e2eeBackupFeatureDisabled returns a standard -32601 error when the E2EE
// backup feature flag is off.
func e2eeBackupFeatureDisabled() *ErrorObj {
	return &ErrorObj{Code: MethodNotFound, Message: "Feature disabled: e2ee_backup"}
}

func (s *Server) handleE2EECreateBackup(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.e2eeBackupEnabled {
		return nil, e2eeBackupFeatureDisabled()
	}

	var params struct {
		RecoveryPhrase []string `json:"recovery_phrase"`
		EncryptedKey   string   `json:"encrypted_key"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: fmt.Sprintf("invalid parameters: %v", err)}
	}

	if len(params.RecoveryPhrase) != 24 {
		return nil, &ErrorObj{Code: InvalidParams, Message: "recovery_phrase must be 24 words"}
	}
	if params.EncryptedKey == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "encrypted_key is required"}
	}

	encryptedKey, err := base64.StdEncoding.DecodeString(params.EncryptedKey)
	if err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: fmt.Sprintf("invalid base64 encrypted_key: %v", err)}
	}

	userID := s.resolveUserID(ctx)
	if userID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "user_id is required"}
	}

	if s.backupMgr == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "backup manager not configured"}
	}

	backupID, err := s.backupMgr.CreateBackup(userID, params.RecoveryPhrase, encryptedKey)
	if err != nil {
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{"backup_id": backupID}, nil
}

func (s *Server) handleE2EEDeleteBackup(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.e2eeBackupEnabled {
		return nil, e2eeBackupFeatureDisabled()
	}

	var params struct {
		BackupID string `json:"backup_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: fmt.Sprintf("invalid parameters: %v", err)}
	}

	if params.BackupID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "backup_id is required"}
	}

	userID := s.resolveUserID(ctx)
	if userID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "user_id is required"}
	}

	if s.backupMgr == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "backup manager not configured"}
	}

	if err := s.backupMgr.DeleteBackup(userID, params.BackupID); err != nil {
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{"deleted": true}, nil
}

func (s *Server) handleE2EEBackupExists(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.e2eeBackupEnabled {
		return nil, e2eeBackupFeatureDisabled()
	}

	var params struct {
		BackupID string `json:"backup_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{Code: InvalidParams, Message: fmt.Sprintf("invalid parameters: %v", err)}
	}

	if params.BackupID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "backup_id is required"}
	}

	userID := s.resolveUserID(ctx)
	if userID == "" {
		return nil, &ErrorObj{Code: InvalidParams, Message: "user_id is required"}
	}

	if s.backupMgr == nil {
		return nil, &ErrorObj{Code: InternalError, Message: "backup manager not configured"}
	}

	exists, err := s.backupMgr.BackupExists(userID, params.BackupID)
	if err != nil {
		return nil, &ErrorObj{Code: InternalError, Message: err.Error()}
	}

	return map[string]interface{}{"exists": exists}, nil
}

// resolveUserID extracts a user ID from the RPC context. It prefers the
// Matrix adapter identity but falls back to peer credentials.
func (s *Server) resolveUserID(ctx context.Context) string {
	if !isInterfaceNil(s.matrix) {
		if uid := s.matrix.GetUserID(); uid != "" {
			return uid
		}
	}
	if cred := PeerCredFromContext(ctx); cred != nil {
		return fmt.Sprintf("uid:%d", cred.UID)
	}
	return ""
}
