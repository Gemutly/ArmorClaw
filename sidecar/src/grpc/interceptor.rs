use prometheus::{Histogram, IntCounter, Registry};
use std::sync::Arc;
use std::time::Instant;
use tonic::{metadata::MetadataMap, service::Interceptor as TonicInterceptor, Code, Status};
use tracing::{debug, error, info, warn};

use crate::security::token::{validate_token, TokenError};

#[derive(Debug, Clone, PartialEq)]
pub struct Version {
    pub major: u32,
    pub minor: u32,
    pub patch: u32,
}

impl Version {
    pub fn parse(version_str: &str) -> Result<Self, String> {
        let parts: Vec<&str> = version_str.split('.').collect();
        if parts.len() != 3 {
            return Err(format!(
                "invalid version format: {} (expected MAJOR.MINOR.PATCH)",
                version_str
            ));
        }

        let major = parts[0]
            .parse::<u32>()
            .map_err(|e| format!("invalid major version: {}", e))?;
        let minor = parts[1]
            .parse::<u32>()
            .map_err(|e| format!("invalid minor version: {}", e))?;
        let patch = parts[2]
            .parse::<u32>()
            .map_err(|e| format!("invalid patch version: {}", e))?;

        Ok(Version {
            major,
            minor,
            patch,
        })
    }

    pub fn compare(&self, other: &Version) -> std::cmp::Ordering {
        match self.major.cmp(&other.major) {
            std::cmp::Ordering::Equal => {}
            ord => return ord,
        }
        match self.minor.cmp(&other.minor) {
            std::cmp::Ordering::Equal => {}
            ord => return ord,
        }
        self.patch.cmp(&other.patch)
    }

    pub fn is_compatible(&self, min: &Version, max: &Version) -> bool {
        self.compare(min) != std::cmp::Ordering::Less
            && self.compare(max) != std::cmp::Ordering::Greater
    }
}

impl std::fmt::Display for Version {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}.{}.{}", self.major, self.minor, self.patch)
    }
}

const METADATA_TOKEN_KEY: &str = "x-request-token";
const METADATA_REQUEST_ID_KEY: &str = "request-id";
const METADATA_TIMESTAMP_KEY: &str = "timestamp";
const METADATA_OPERATION_KEY: &str = "operation";
const METADATA_VERSION_KEY: &str = "x-sidecar-version";
const METADATA_SERVER_VERSION_KEY: &str = "x-sidecar-server-version";

const SERVER_VERSION: &str = "1.0.0";
const MIN_CLIENT_VERSION: &str = "1.0.0";
const MAX_CLIENT_VERSION: &str = "1.5.0";

/// Security interceptor that validates ephemeral tokens on every request.
///
/// This interceptor:
/// - Extracts and validates the ephemeral token from request metadata
/// - Logs request details (operation, request_id) without sensitive data
/// - Collects metrics (request count, latency, errors)
/// - Rejects requests without valid tokens with UNAUTHENTICATED status
#[derive(Clone)]
pub struct SecurityInterceptor {
    shared_secret: Arc<Vec<u8>>,
    requests_total: IntCounter,
    request_errors_total: IntCounter,
    request_duration: Histogram,
}

impl SecurityInterceptor {
    /// Creates a new security interceptor with the given shared secret and metrics registry.
    ///
    /// # Arguments
    /// * `shared_secret` - The shared secret used for HMAC token validation
    /// * `registry` - The Prometheus metrics registry
    pub fn new(shared_secret: Vec<u8>, registry: &Registry) -> Result<Self, prometheus::Error> {
        let requests_total = IntCounter::new(
            "armorclaw_sidecar_requests_total",
            "Total number of requests received by the sidecar",
        )?;

        let request_errors_total = IntCounter::new(
            "armorclaw_sidecar_request_errors_total",
            "Total number of request errors",
        )?;

        let request_duration = Histogram::with_opts(
            prometheus::HistogramOpts::new(
                "armorclaw_sidecar_request_duration_seconds",
                "Request duration in seconds",
            )
            .buckets(vec![
                0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
            ]),
        )?;

        registry.register(Box::new(requests_total.clone()))?;
        registry.register(Box::new(request_errors_total.clone()))?;
        registry.register(Box::new(request_duration.clone()))?;

        Ok(Self {
            shared_secret: Arc::new(shared_secret),
            requests_total,
            request_errors_total,
            request_duration,
        })
    }

    /// Validates a token from metadata and returns the token info if valid.
    ///
    /// # Arguments
    /// * `metadata` - The gRPC request metadata
    ///
    /// # Returns
    /// * `Ok(String)` - The validated operation name
    /// * `Err(Status)` - UNAUTHENTICATED if validation fails
    fn validate_metadata(&self, metadata: &MetadataMap) -> Result<String, Status> {
        let token = metadata
            .get(METADATA_TOKEN_KEY)
            .and_then(|value| value.to_str().ok())
            .ok_or_else(|| {
                warn!("Request rejected: missing token metadata");
                Status::new(Code::Unauthenticated, "missing authentication token")
            })?;

        let token_info = validate_token(token, &self.shared_secret).map_err(|e| {
            let error_msg = match e {
                TokenError::InvalidFormat(n) => {
                    warn!("Request rejected: invalid token format ({} parts)", n);
                    format!("invalid token format: expected 4 parts, got {}", n)
                }
                TokenError::InvalidTimestamp(_) => {
                    warn!("Request rejected: invalid timestamp");
                    "invalid timestamp".to_string()
                }
                TokenError::TokenTooOld(_) => {
                    warn!("Request rejected: token timestamp is too old");
                    "token timestamp is too old".to_string()
                }
                TokenError::TokenExpired(_) => {
                    warn!("Request rejected: token has expired");
                    "token has expired".to_string()
                }
                TokenError::InvalidSignature => {
                    warn!("Request rejected: invalid HMAC signature");
                    "invalid token signature".to_string()
                }
                TokenError::HmacError(msg) => {
                    error!("HMAC validation error: {}", msg);
                    format!("HMAC error: {}", msg)
                }
            };
            Status::new(Code::Unauthenticated, error_msg)
        })?;

        debug!(
            request_id = %token_info.request_id,
            operation = %token_info.operation,
            timestamp = %token_info.timestamp,
            "Token validated successfully"
        );

        self.validate_client_version(metadata)?;

        Ok(token_info.operation)
    }

    fn validate_client_version(&self, metadata: &MetadataMap) -> Result<(), Status> {
        let client_version_str = metadata
            .get(METADATA_VERSION_KEY)
            .and_then(|value| value.to_str().ok());

        if let Some(version_str) = client_version_str {
            let client_version = Version::parse(version_str).map_err(|e| {
                warn!("Request rejected: invalid client version {}", version_str);
                Status::new(Code::InvalidArgument, e)
            })?;

            let min_version = Version::parse(MIN_CLIENT_VERSION).unwrap();
            let max_version = Version::parse(MAX_CLIENT_VERSION).unwrap();

            if !client_version.is_compatible(&min_version, &max_version) {
                warn!(
                    client_version = %client_version,
                    min_version = %min_version,
                    max_version = %max_version,
                    "Request rejected: incompatible client version"
                );
                return Err(Status::new(
                    Code::FailedPrecondition,
                    format!(
                        "incompatible client version {} (supported: {}-{})",
                        client_version, min_version, max_version
                    ),
                ));
            }

            debug!(
                client_version = %client_version,
                "Client version validated"
            );
        } else {
            debug!("No client version provided, skipping validation");
        }

        Ok(())
    }

    /// Extracts safe request information for logging (no sensitive data).
    ///
    /// # Arguments
    /// * `metadata` - The gRPC request metadata
    ///
    /// # Returns
    /// A tuple of (request_id, operation) if present, or defaults
    fn extract_request_info(metadata: &MetadataMap) -> (String, String) {
        let request_id = metadata
            .get(METADATA_REQUEST_ID_KEY)
            .and_then(|v| v.to_str().ok())
            .map(|s| s.to_string())
            .unwrap_or_else(|| "unknown".to_string());

        let operation = metadata
            .get(METADATA_OPERATION_KEY)
            .and_then(|v| v.to_str().ok())
            .map(|s| s.to_string())
            .unwrap_or_else(|| "unknown".to_string());

        (request_id, operation)
    }
}

/// Implements the tonic Interceptor trait to intercept all gRPC requests.
impl TonicInterceptor for SecurityInterceptor {
    fn call(&mut self, req: tonic::Request<()>) -> Result<tonic::Request<()>, Status> {
        let start_time = Instant::now();
        let metadata = req.metadata();
        let method = "unknown";

        self.requests_total.inc();

        let (request_id, operation) = Self::extract_request_info(metadata);

        match self.validate_metadata(metadata) {
            Ok(_) => {
                info!(
                    method = %method,
                    request_id = %request_id,
                    operation = %operation,
                    "Request authorized"
                );

                let latency = start_time.elapsed();
                self.request_duration.observe(latency.as_secs_f64());

                debug!(
                    method = %method,
                    request_id = %request_id,
                    latency_ms = latency.as_millis(),
                    "Request completed"
                );

                Ok(req)
            }
            Err(e) => {
                self.request_errors_total.inc();
                warn!(
                    method = %method,
                    request_id = %request_id,
                    error = %e.message(),
                    "Request failed authentication"
                );
                Err(e)
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::security::token::parse_token;
    use hmac::{Hmac, Mac};
    use sha2::Sha256;
    use std::time::{SystemTime, UNIX_EPOCH};
    use tonic::metadata::MetadataValue;

    type HmacSha256 = Hmac<Sha256>;

    #[test]
    fn test_security_interceptor_creation() {
        let secret = b"test-secret-key-32-bytes-long!".to_vec();
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(secret, &registry);
        assert!(interceptor.is_ok());
        assert_eq!(Arc::strong_count(&interceptor.unwrap().shared_secret), 1);
    }

    #[test]
    fn test_extract_request_info_with_metadata() {
        let secret = b"test-secret-key-32-bytes-long!".to_vec();
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(secret, &registry).unwrap();

        let mut metadata = MetadataMap::new();
        metadata.insert(
            METADATA_REQUEST_ID_KEY,
            MetadataValue::try_from("test-request-id").unwrap(),
        );
        metadata.insert(
            METADATA_OPERATION_KEY,
            MetadataValue::try_from("test-operation").unwrap(),
        );

        let (request_id, operation) = SecurityInterceptor::extract_request_info(&metadata);
        assert_eq!(request_id, "test-request-id");
        assert_eq!(operation, "test-operation");
    }

    #[test]
    fn test_extract_request_info_without_metadata() {
        let secret = b"test-secret-key-32-bytes-long!".to_vec();
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(secret, &registry).unwrap();
        let metadata = MetadataMap::new();

        let (request_id, operation) = SecurityInterceptor::extract_request_info(&metadata);
        assert_eq!(request_id, "unknown");
        assert_eq!(operation, "unknown");
    }

    #[test]
    fn test_validate_metadata_missing_token() {
        let secret = b"test-secret-key-32-bytes-long!".to_vec();
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(secret, &registry).unwrap();
        let metadata = MetadataMap::new();

        let result = interceptor.validate_metadata(&metadata);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code(), Code::Unauthenticated);
        assert!(err.message().contains("missing authentication token"));
    }

    #[test]
    fn test_validate_metadata_valid_token() {
        let shared_secret = b"test-secret-key-32-bytes-long!";
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(shared_secret.to_vec(), &registry).unwrap();

        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;

        let request_id = "test-request-id";
        let operation = "test-operation";

        let data_to_sign = format!("{}{}{}", request_id, now, operation);
        let mut mac = HmacSha256::new_from_slice(shared_secret).unwrap();
        mac.update(data_to_sign.as_bytes());
        let signature = hex::encode(mac.finalize().into_bytes());

        let token = format!("{}:{}:{}:{}", request_id, now, operation, signature);

        let mut metadata = MetadataMap::new();
        metadata.insert(
            METADATA_TOKEN_KEY,
            MetadataValue::try_from(token.as_str()).unwrap(),
        );

        let result = interceptor.validate_metadata(&metadata);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), operation);
    }

    #[test]
    fn test_validate_metadata_invalid_signature() {
        let shared_secret = b"test-secret-key-32-bytes-long!";
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(shared_secret.to_vec(), &registry).unwrap();

        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;

        let token = format!(
            "{}:{}:{}:{}",
            "test-request-id", now, "test-operation", "invalid-signature"
        );

        let mut metadata = MetadataMap::new();
        metadata.insert(
            METADATA_TOKEN_KEY,
            MetadataValue::try_from(token.as_str()).unwrap(),
        );

        let result = interceptor.validate_metadata(&metadata);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code(), Code::Unauthenticated);
        assert!(err.message().contains("invalid token signature"));
    }

    #[test]
    fn test_validate_metadata_expired_token() {
        let shared_secret = b"test-secret-key-32-bytes-long!";
        let registry = Registry::new();
        let interceptor = SecurityInterceptor::new(shared_secret.to_vec(), &registry).unwrap();

        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;

        let timestamp = now - 2000;
        let request_id = "test-request-id";
        let operation = "test-operation";

        let data_to_sign = format!("{}{}{}", request_id, timestamp, operation);
        let mut mac = HmacSha256::new_from_slice(shared_secret).unwrap();
        mac.update(data_to_sign.as_bytes());
        let signature = hex::encode(mac.finalize().into_bytes());

        let token = format!("{}:{}:{}:{}", request_id, timestamp, operation, signature);

        let mut metadata = MetadataMap::new();
        metadata.insert(
            METADATA_TOKEN_KEY,
            MetadataValue::try_from(token.as_str()).unwrap(),
        );

        let result = interceptor.validate_metadata(&metadata);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code(), Code::Unauthenticated);
        assert!(err.message().contains("too old"));
    }
}
