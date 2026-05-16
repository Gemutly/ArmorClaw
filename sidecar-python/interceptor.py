"""HMAC-SHA256 token validation and version interceptor for ArmorClaw Python sidecar."""

import hmac
import hashlib
import os
import time
from typing import Optional

import grpc


# Token constants (must match Go bridge/pkg/sidecar/token.go)
TOKEN_TTL_SECONDS = 30 * 60       # 30 minutes
MAX_TIMESTAMP_AGE_SECONDS = 5 * 60  # 5 minutes

# Version constants (must match Go bridge/pkg/sidecar/version.go)
SERVER_VERSION = "1.0.0"
VERSION_METADATA_KEY = "x-sidecar-version"
SERVER_VERSION_METADATA_KEY = "x-sidecar-server-version"
TOKEN_METADATA_KEY = "x-request-token"


def load_shared_secret(max_retries: int = 30, retry_interval: float = 1.0) -> str:
    """Read HMAC shared secret from a tmpfs-backed file mounted by the Go Bridge.

    The Go Bridge:
    1. Mounts tmpfs at /run/armorclaw/secrets/ (RAM-only, no disk persistence)
    2. Writes the shared secret to /run/armorclaw/secrets/office-hmac
    3. Docker Compose mounts this file read-only into the container at /run/secrets/shared_secret

    The secret never touches persistent storage. It is never exposed via environment variables.

    Retries up to max_retries times with retry_interval sleep between attempts
    to handle init-service mount timing races.
    """
    secret_path = os.environ.get("SECRET_PATH", "/run/secrets/shared_secret")
    last_err = None
    for attempt in range(1, max_retries + 1):
        try:
            with open(secret_path, "r") as f:
                secret = f.read().strip()
            if not secret:
                raise RuntimeError(f"Shared secret is empty or missing at {secret_path}")
            return secret
        except (FileNotFoundError, PermissionError, RuntimeError) as exc:
            last_err = exc
            if attempt < max_retries:
                print(f"load_shared_secret: attempt {attempt}/{max_retries} failed ({exc}); retrying in {retry_interval}s")
                time.sleep(retry_interval)
    raise last_err


def _calculate_hmac(shared_secret: str, data: str) -> str:
    """Compute HMAC-SHA256 and return hex digest. Matches Go calculateHMAC()."""
    return hmac.new(
        shared_secret.encode("utf-8"),
        data.encode("utf-8"),
        hashlib.sha256,
    ).hexdigest()


def validate_token(shared_secret: str, token: str) -> None:
    """Validate a token string. Raises ValueError on any validation failure.

    Token format: {request_id}:{timestamp}:{operation}:{hmac_hex_signature}
    Validation order (matches Go ValidateToken):
    1. Parse into 4 parts
    2. Check timestamp age <= 5 minutes (MaxTimestampAge)
    3. Check token TTL <= 30 minutes (TokenTTL)
    4. Recompute HMAC and compare with constant-time comparison
    """
    parts = token.split(":")
    if len(parts) != 4:
        raise ValueError(f"malformed token: expected 4 parts, got {len(parts)}")

    request_id, timestamp_str, operation, signature = parts

    try:
        timestamp = int(timestamp_str)
    except ValueError:
        raise ValueError(f"malformed token: invalid timestamp '{timestamp_str}'")

    now = int(time.time())

    # Check timestamp age (MaxTimestampAge = 5 minutes)
    if (now - timestamp) > MAX_TIMESTAMP_AGE_SECONDS:
        raise ValueError(
            f"token timestamp is too old (> {MAX_TIMESTAMP_AGE_SECONDS}s)"
        )

    # Check token TTL (30 minutes from timestamp)
    expiration = timestamp + TOKEN_TTL_SECONDS
    if now > expiration:
        raise ValueError(
            f"token has expired (TTL: {TOKEN_TTL_SECONDS}s)"
        )

    # Recompute HMAC: HMAC-SHA256(shared_secret, "{request_id}{timestamp}{operation}")
    data_to_sign = f"{request_id}{timestamp}{operation}"
    expected_signature = _calculate_hmac(shared_secret, data_to_sign)

    # Constant-time comparison (matches Go hmac.Equal)
    if not hmac.compare_digest(signature, expected_signature):
        raise ValueError("invalid token signature")


class TokenInterceptor(grpc.ServerInterceptor):
    """Synchronous gRPC server interceptor that validates HMAC tokens and injects version metadata.

    Compatible with grpc.server() (sync) used by worker.py.
    """

    def __init__(self, shared_secret: str):
        self._shared_secret = shared_secret

    def intercept_service(self, continuation, handler_call_details):
        """Intercept incoming gRPC calls to validate token and inject version."""
        metadata = dict(handler_call_details.invocation_metadata or [])
        token = metadata.get(TOKEN_METADATA_KEY)

        if not token:
            return self._abort_unauthenticated("missing token")

        try:
            validate_token(self._shared_secret, token)
        except ValueError as e:
            return self._abort_unauthenticated(str(e))

        # Token valid — proceed to the actual handler
        handler = continuation(handler_call_details)

        if handler is None:
            return None

        # Wrap the handler to inject server version in initial metadata
        return self._wrap_handler_with_version(handler)

    def _abort_unauthenticated(self, reason: str):
        """Return an aborting handler for UNAUTHENTICATED responses."""
        def abort_handler(request, context):
            context.abort(grpc.StatusCode.UNAUTHENTICATED, reason)

        return grpc.unary_unary_rpc_method_handler(abort_handler)

    def _wrap_handler_with_version(self, handler):
        """Wrap the gRPC handler to inject x-sidecar-server-version in initial metadata."""
        original_unary = handler.unary_unary
        original_stream = handler.unary_stream

        if original_unary:

            def wrapped_unary(request, context):
                context.set_trailing_metadata((
                    (SERVER_VERSION_METADATA_KEY, SERVER_VERSION),
                ))
                return original_unary(request, context)

            return grpc.unary_unary_rpc_method_handler(
                wrapped_unary,
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        if original_stream:

            def wrapped_stream(request, context):
                context.set_trailing_metadata((
                    (SERVER_VERSION_METADATA_KEY, SERVER_VERSION),
                ))
                for response in original_stream(request, context):
                    yield response

            return grpc.unary_stream_rpc_method_handler(
                wrapped_stream,
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        return handler
