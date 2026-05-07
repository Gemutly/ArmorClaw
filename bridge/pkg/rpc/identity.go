package rpc

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const peerCredKey contextKey = "peer_credentials"

// PeerCredentials holds Unix socket peer identity fields.
type PeerCredentials struct {
	UID int32
	GID int32
	PID int32
}

// ContextWithPeerCred stores peer credentials in a context.
func ContextWithPeerCred(parent context.Context, cred *PeerCredentials) context.Context {
	return context.WithValue(parent, peerCredKey, cred)
}

// PeerCredFromContext retrieves peer credentials from a context.
// Returns nil if not present.
func PeerCredFromContext(ctx context.Context) *PeerCredentials {
	if v, ok := ctx.Value(peerCredKey).(*PeerCredentials); ok {
		return v
	}
	return nil
}

// resolveClientIdentity determines a rate-limiting identity string from an HTTP request.
func resolveClientIdentity(r *http.Request, serverMode string) string {
	if serverMode == "native" {
		if cred := PeerCredFromContext(r.Context()); cred != nil {
			return "uid:" + itoa(int64(cred.UID))
		}
	}

	if serverMode == "sentinel" || serverMode == "cloudflare" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}

	return r.RemoteAddr
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}
