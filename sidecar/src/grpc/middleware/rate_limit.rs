//! Token bucket rate limiting middleware for gRPC services.
//!
//! This module implements a thread-safe token bucket algorithm using atomic operations
//! to enforce rate limits on incoming gRPC requests.
//!
//! # Rate Limiting Parameters
//! - **Default Rate**: 100 requests per second
//! - **Default Burst**: 200 requests (maximum concurrent tokens)
//!
//! # Algorithm
//! The token bucket algorithm works as follows:
//! 1. The bucket has a maximum capacity (burst size)
//! 2. Tokens are added to the bucket at a fixed rate (rate limit)
//! 3. Each request consumes one token
//! 4. If no tokens are available, the request is rate limited

use prometheus::{IntCounter, Registry};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use tonic::{
    service::Interceptor as TonicInterceptor,
    Status,
};
use tracing::{debug, warn};

/// Token bucket rate limiter using atomic operations.
///
/// This struct maintains the state of the token bucket using atomic operations
/// to ensure thread safety without mutex locks.
#[derive(Clone)]
pub struct TokenBucket {
    /// Maximum number of tokens the bucket can hold (burst capacity)
    burst_capacity: u64,
    /// Number of tokens added per second (rate limit)
    tokens_per_second: u64,
    /// Current number of tokens in the bucket (stored as atomic)
    tokens: Arc<AtomicU64>,
    /// Last time tokens were calculated (nanoseconds since UNIX_EPOCH)
    last_update: Arc<AtomicU64>,
    /// Total number of requests that were rate limited
    rate_limited_total: IntCounter,
}

impl TokenBucket {
    /// Creates a new token bucket with the specified parameters.
    ///
    /// # Arguments
    /// * `tokens_per_second` - Rate limit: number of tokens added per second
    /// * `burst_capacity` - Maximum number of tokens the bucket can hold
    ///
    /// # Returns
    /// A new TokenBucket instance
    pub fn new(tokens_per_second: u64, burst_capacity: u64) -> Self {
        Self {
            burst_capacity,
            tokens_per_second,
            tokens: Arc::new(AtomicU64::new(burst_capacity)),
            last_update: Arc::new(AtomicU64::new(Self::current_nanos())),
            rate_limited_total: IntCounter::new(
                "armorclaw_sidecar_rate_limited_total",
                "Total number of requests that were rate limited",
            )
            .unwrap(),
        }
    }

    /// Creates a new token bucket with Prometheus metrics registration.
    ///
    /// # Arguments
    /// * `tokens_per_second` - Rate limit: number of tokens added per second
    /// * `burst_capacity` - Maximum number of tokens the bucket can hold
    /// * `registry` - Prometheus metrics registry
    ///
    /// # Returns
    /// A new TokenBucket instance with registered metrics
    pub fn new_with_registry(
        tokens_per_second: u64,
        burst_capacity: u64,
        registry: &Registry,
    ) -> Result<Self, prometheus::Error> {
        let bucket = Self::new(tokens_per_second, burst_capacity);
        registry.register(Box::new(bucket.rate_limited_total.clone()))?;
        Ok(bucket)
    }

    /// Gets the current time in nanoseconds since UNIX_EPOCH.
    fn current_nanos() -> u64 {
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos() as u64
    }

    /// Attempts to consume a token from the bucket.
    ///
    /// This method:
    /// 1. Calculates elapsed time since last update
    /// 2. Adds new tokens based on elapsed time
    /// 3. Attempts to consume a token atomically
    ///
    /// # Returns
    /// * `true` if a token was successfully consumed
    /// * `false` if the bucket is empty (rate limited)
    #[inline]
    pub fn try_acquire(&self) -> bool {
        let now = Self::current_nanos();
        let last_update = self.last_update.load(Ordering::Relaxed);

        if now > last_update {
            // Calculate elapsed time and add new tokens
            let elapsed_nanos = now - last_update;
            let elapsed_seconds = elapsed_nanos as f64 / 1_000_000_000.0;
            let new_tokens = (elapsed_seconds * self.tokens_per_second as f64) as u64;

            if new_tokens > 0 {
                // Try to update the last_update time
                let last_update_ref = &self.last_update;
                let _ = last_update_ref.compare_exchange(
                    last_update,
                    now,
                    Ordering::Relaxed,
                    Ordering::Relaxed,
                );

                // Atomically add new tokens, capped at burst capacity
                let tokens_ref = &self.tokens;
                loop {
                    let current_tokens = tokens_ref.load(Ordering::Relaxed);
                    let added_tokens = (current_tokens + new_tokens).min(self.burst_capacity);

                    match tokens_ref.compare_exchange(
                        current_tokens,
                        added_tokens,
                        Ordering::Relaxed,
                        Ordering::Relaxed,
                    ) {
                        Ok(_) => break,
                        Err(_) => continue, // Retry if CAS failed
                    }
                }
            }
        }

        // Try to consume a token atomically
        let tokens_ref = &self.tokens;
        loop {
            let current_tokens = tokens_ref.load(Ordering::Relaxed);

            if current_tokens == 0 {
                self.rate_limited_total.inc();
                return false;
            }

            match tokens_ref.compare_exchange(
                current_tokens,
                current_tokens - 1,
                Ordering::Relaxed,
                Ordering::Relaxed,
            ) {
                Ok(_) => return true,
                Err(_) => continue, // Retry if CAS failed
            }
        }
    }

    /// Gets the current number of tokens in the bucket.
    ///
    /// # Returns
    /// The current token count
    #[cfg(test)]
    pub fn current_tokens(&self) -> u64 {
        self.tokens.load(Ordering::Relaxed)
    }

    /// Gets the total number of requests that were rate limited.
    ///
    /// # Returns
    /// The total count of rate-limited requests
    pub fn rate_limited_count(&self) -> u64 {
        self.rate_limited_total.get()
    }
}

/// gRPC interceptor that applies token bucket rate limiting.
///
/// This interceptor wraps a TokenBucket and applies it to all incoming gRPC requests,
/// rejecting requests that exceed the rate limit with RESOURCE_EXHAUSTED status.
#[derive(Clone)]
pub struct RateLimitInterceptor {
    bucket: TokenBucket,
}

impl RateLimitInterceptor {
    /// Creates a new rate limiting interceptor with default parameters (100 req/s, burst 200).
    ///
    /// # Returns
    /// A new RateLimitInterceptor instance
    pub fn new() -> Self {
        Self {
            bucket: TokenBucket::new(100, 200),
        }
    }

    /// Creates a new rate limiting interceptor with custom parameters.
    ///
    /// # Arguments
    /// * `tokens_per_second` - Rate limit: number of tokens added per second
    /// * `burst_capacity` - Maximum number of tokens the bucket can hold
    ///
    /// # Returns
    /// A new RateLimitInterceptor instance
    pub fn with_params(tokens_per_second: u64, burst_capacity: u64) -> Self {
        Self {
            bucket: TokenBucket::new(tokens_per_second, burst_capacity),
        }
    }

    /// Creates a new rate limiting interceptor with Prometheus metrics registration.
    ///
    /// # Arguments
    /// * `tokens_per_second` - Rate limit: number of tokens added per second
    /// * `burst_capacity` - Maximum number of tokens the bucket can hold
    /// * `registry` - Prometheus metrics registry
    ///
    /// # Returns
    /// A new RateLimitInterceptor instance with registered metrics
    pub fn with_registry(
        tokens_per_second: u64,
        burst_capacity: u64,
        registry: &Registry,
    ) -> Result<Self, prometheus::Error> {
        let bucket = TokenBucket::new_with_registry(tokens_per_second, burst_capacity, registry)?;
        Ok(Self { bucket })
    }

    /// Gets the total number of requests that were rate limited.
    ///
    /// # Returns
    /// The total count of rate-limited requests
    pub fn rate_limited_count(&self) -> u64 {
        self.bucket.rate_limited_count()
    }
}

impl Default for RateLimitInterceptor {
    fn default() -> Self {
        Self::new()
    }
}

impl TonicInterceptor for RateLimitInterceptor {
    fn call(&mut self, req: tonic::Request<()>) -> Result<tonic::Request<()>, Status> {
        let method = "unknown";

        if !self.bucket.try_acquire() {
            warn!(
                method = %method,
                "Request rate limited - token bucket exhausted"
            );
            return Err(Status::resource_exhausted("rate limit exceeded"));
        }

        debug!(
            method = %method,
            "Request allowed - token acquired"
        );

        Ok(req)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;
    use tonic::Code;

    #[test]
    fn test_token_bucket_creation() {
        let bucket = TokenBucket::new(100, 200);
        assert_eq!(bucket.burst_capacity, 200);
        assert_eq!(bucket.tokens_per_second, 100);
        assert_eq!(bucket.current_tokens(), 200);
    }

    #[test]
    fn test_token_bucket_single_acquire() {
        let bucket = TokenBucket::new(100, 200);
        assert!(bucket.try_acquire());
        assert_eq!(bucket.current_tokens(), 199);
    }

    #[test]
    fn test_token_bucket_burst_acquire() {
        let bucket = TokenBucket::new(100, 200);

        // Acquire all burst tokens
        for _ in 0..200 {
            assert!(bucket.try_acquire());
        }

        assert_eq!(bucket.current_tokens(), 0);
        assert!(!bucket.try_acquire()); // Should fail - bucket empty
    }

    #[test]
    fn test_token_bucket_token_replenishment() {
        let bucket = TokenBucket::new(100, 10); // Low burst for faster test

        // Acquire all tokens
        for _ in 0..10 {
            assert!(bucket.try_acquire());
        }

        assert_eq!(bucket.current_tokens(), 0);
        assert!(!bucket.try_acquire());

        // Wait for tokens to replenish (1 second for 100 tokens/second)
        std::thread::sleep(Duration::from_millis(1100));

        // Should have new tokens now
        assert!(bucket.try_acquire());
    }

    #[test]
    fn test_token_bucket_capped_at_burst() {
        let bucket = TokenBucket::new(100, 10);

        // Wait long enough to accumulate more than burst capacity
        std::thread::sleep(Duration::from_millis(500));

        // Tokens should be capped at burst capacity
        assert!(bucket.current_tokens() <= 10);
    }

    #[test]
    fn test_token_bucket_concurrent_acquires() {
        let bucket = Arc::new(TokenBucket::new(100, 1000));
        let mut handles = vec![];

        // Spawn multiple threads to test concurrent access
        for _ in 0..10 {
            let bucket_clone = Arc::clone(&bucket);
            let handle = std::thread::spawn(move || {
                let mut success_count = 0;
                for _ in 0..100 {
                    if bucket_clone.try_acquire() {
                        success_count += 1;
                    }
                }
                success_count
            });
            handles.push(handle);
        }

        let total_success: usize = handles.into_iter().map(|h| h.join().unwrap()).sum();
        assert_eq!(total_success, 1000); // All burst tokens should be consumed
        assert_eq!(bucket.current_tokens(), 0);
    }

    #[test]
    fn test_rate_limit_interceptor_creation() {
        let interceptor = RateLimitInterceptor::new();
        assert!(interceptor.bucket.current_tokens() == 200);
    }

    #[test]
    fn test_rate_limit_interceptor_with_params() {
        let interceptor = RateLimitInterceptor::with_params(50, 100);
        assert_eq!(interceptor.bucket.burst_capacity, 100);
        assert_eq!(interceptor.bucket.tokens_per_second, 50);
    }

    #[test]
    fn test_rate_limit_interceptor_default() {
        let interceptor = RateLimitInterceptor::default();
        assert_eq!(interceptor.bucket.burst_capacity, 200);
        assert_eq!(interceptor.bucket.tokens_per_second, 100);
    }

    #[test]
    fn test_rate_limit_interceptor_with_registry() {
        let registry = Registry::new();
        let interceptor = RateLimitInterceptor::with_registry(50, 100, &registry);
        assert!(interceptor.is_ok());

        let interceptor = interceptor.unwrap();
        assert_eq!(interceptor.bucket.burst_capacity, 100);
        assert_eq!(interceptor.bucket.tokens_per_second, 50);
    }

    #[test]
    fn test_rate_limit_interceptor_call_allowed() {
        let mut interceptor = RateLimitInterceptor::new();

        // First 200 requests should be allowed
        for _ in 0..200 {
            let result = interceptor.call(tonic::Request::new(()));
            assert!(result.is_ok());
        }

        // 201st request should be rate limited
        let result = interceptor.call(tonic::Request::new(()));
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code(), Code::ResourceExhausted);
    }

    #[test]
    fn test_rate_limit_interceptor_with_registry_rate_limiting() {
        let registry = Registry::new();
        let mut interceptor = RateLimitInterceptor::with_registry(100, 5, &registry).unwrap();

        // Consume all tokens
        for _ in 0..5 {
            let _ = interceptor.call(tonic::Request::new(()));
        }

        // Next request should be rate limited
        let _ = interceptor.call(tonic::Request::new(()));

        // Check metrics
        assert_eq!(interceptor.rate_limited_count(), 1);
    }

    #[test]
    fn test_rate_limit_interceptor_replenishment() {
        let mut interceptor = RateLimitInterceptor::with_params(100, 10);

        // Consume all tokens
        for _ in 0..10 {
            assert!(interceptor.call(tonic::Request::new(())).is_ok());
        }

        // Next request should fail
        assert!(interceptor.call(tonic::Request::new(())).is_err());

        // Wait for replenishment
        std::thread::sleep(Duration::from_millis(1100));

        // Should succeed now
        assert!(interceptor.call(tonic::Request::new(())).is_ok());
    }
}
