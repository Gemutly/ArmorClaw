package sidecar

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// HMACClientInterceptor returns a gRPC unary client interceptor that generates
// HMAC-SHA256 auth tokens for every outgoing request to a sidecar.
// Tokens are sent via the "x-request-token" metadata key, matching the
// Rust and Python sidecar interceptor expectations.
func HMACClientInterceptor(gen *TokenGenerator) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		requestID, err := GenerateRequestID()
		if err != nil {
			return fmt.Errorf("hmac request-id generation failed: %w", err)
		}

		token, _, err := gen.GenerateToken(requestID, method)
		if err != nil {
			return fmt.Errorf("hmac token generation failed: %w", err)
		}

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		md.Set("x-request-token", token)

		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// HMACStreamClientInterceptor returns a gRPC stream client interceptor that
// generates HMAC-SHA256 auth tokens for every outgoing stream request.
func HMACStreamClientInterceptor(gen *TokenGenerator) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {

		requestID, err := GenerateRequestID()
		if err != nil {
			return nil, fmt.Errorf("hmac request-id generation failed: %w", err)
		}

		token, _, err := gen.GenerateToken(requestID, method)
		if err != nil {
			return nil, fmt.Errorf("hmac token generation failed: %w", err)
		}

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		md.Set("x-request-token", token)

		ctx = metadata.NewOutgoingContext(ctx, md)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// InitTokenGenerator reads the HMAC secret and creates a TokenGenerator.
// Returns nil (no error) if the secret cannot be read — sidecar auth will be
// disabled and requests will rely on SKIP_TOKEN_VALIDATION until T7 removes it.
func InitTokenGenerator(logger *slog.Logger) *TokenGenerator {
	secret, err := ReadHMACSecret(DefaultHMACSecretPath)
	if err != nil {
		if logger != nil {
			logger.Warn("HMAC secret not available — sidecar auth disabled",
				"path", DefaultHMACSecretPath,
				"error", err,
			)
		}
		return nil
	}

	gen, err := NewTokenGenerator([]byte(secret))
	if err != nil {
		if logger != nil {
			logger.Warn("HMAC token generator init failed — sidecar auth disabled", "error", err)
		}
		return nil
	}

	if logger != nil {
		logger.Info("HMAC token generator initialized for sidecar auth",
			"path", DefaultHMACSecretPath,
		)
	}
	return gen
}
