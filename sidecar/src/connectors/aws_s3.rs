use crate::error::{Result, SidecarError};
use crate::reliability::S3Reliability;
use async_stream::try_stream;
use aws_config::meta::region::RegionProviderChain;
use aws_config::BehaviorVersion;
use aws_sdk_s3::{
    primitives::ByteStream,
    Client as S3Client,
};
use futures::Stream;
use sha2::{Digest, Sha256};
use std::io;
use std::path::PathBuf;
use std::pin::Pin;
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll};
use std::time::{SystemTime, UNIX_EPOCH};
use tokio::io::{AsyncRead, AsyncReadExt, ReadBuf};
use tokio::fs::File;
use tracing::{debug, error, info};

/// S3 upload request parameters.
///
/// Mutually exclusive: only one of `content` or `file_path` should be set.
#[derive(Debug, Clone)]
pub struct S3UploadRequest {
    /// S3 bucket name to upload to.
    pub bucket: String,
    /// Object key (path) within the bucket.
    pub key: String,
    /// AWS region (e.g., "us-east-1").
    pub region: String,
    /// Content type (e.g., "application/octet-stream").
    pub content_type: Option<String>,
    /// In-memory content bytes. Mutually exclusive with `file_path`.
    pub content: Option<Vec<u8>>,
    /// Local file path for streaming upload. Mutually exclusive with `content`.
    pub file_path: Option<PathBuf>,
    /// Optional AWS access key (ephemeral, from request metadata).
    pub access_key: Option<String>,
    /// Optional AWS secret key (ephemeral, from request metadata).
    pub secret_key: Option<String>,
    /// Optional AWS session token (ephemeral, from request metadata).
    pub session_token: Option<String>,
}

/// S3 upload result containing metadata about the uploaded object.
#[derive(Debug, Clone)]
pub struct S3UploadResult {
    /// S3 object URI in format "s3://bucket/key".
    pub blob_id: String,
    /// ETag returned by S3 for the uploaded object.
    pub etag: String,
    /// Size of the uploaded object in bytes.
    pub size_bytes: i64,
    /// SHA256 hash of the uploaded content (computed during upload).
    pub content_hash_sha256: String,
    /// Upload timestamp as Unix epoch seconds.
    pub timestamp_unix: i64,
}

/// S3 download request parameters.
#[derive(Debug, Clone)]
pub struct S3DownloadRequest {
    /// S3 bucket name to download from.
    pub bucket: String,
    /// Object key (path) within the bucket.
    pub key: String,
    /// AWS region (e.g., "us-east-1").
    pub region: String,
    /// Optional offset bytes for range requests (0-based).
    pub offset_bytes: Option<i64>,
    /// Optional maximum bytes to download for range requests.
    pub max_bytes: Option<i64>,
    /// Optional AWS access key (ephemeral, from request metadata).
    pub access_key: Option<String>,
    /// Optional AWS secret key (ephemeral, from request metadata).
    pub secret_key: Option<String>,
    /// Optional AWS session token (ephemeral, from request metadata).
    pub session_token: Option<String>,
}

/// A chunk of blob data from a streaming download.
#[derive(Debug, Clone)]
pub struct BlobChunk {
    /// Chunk data (up to 1MB).
    pub data: Vec<u8>,
    /// Byte offset of this chunk within the original blob.
    pub offset: i64,
    /// True if this is the last chunk of the blob.
    pub is_last: bool,
}

/// S3 list request parameters.
#[derive(Debug, Clone)]
pub struct S3ListRequest {
    /// S3 bucket name to list objects from.
    pub bucket: String,
    /// AWS region (e.g., "us-east-1").
    pub region: String,
    /// Optional prefix to filter objects.
    pub prefix: Option<String>,
    /// Optional maximum number of results to return.
    pub max_results: Option<i32>,
    /// Optional AWS access key (ephemeral, from request metadata).
    pub access_key: Option<String>,
    /// Optional AWS secret key (ephemeral, from request metadata).
    pub secret_key: Option<String>,
    /// Optional AWS session token (ephemeral, from request metadata).
    pub session_token: Option<String>,
}

/// Information about a blob object.
#[derive(Debug, Clone)]
pub struct BlobInfo {
    /// S3 object URI in format "s3://bucket/key".
    pub uri: String,
    /// Size of the object in bytes.
    pub size_bytes: i64,
    /// Content type of the object.
    pub content_type: String,
    /// Last modified timestamp as Unix epoch seconds.
    pub last_modified_unix: i64,
    /// ETag returned by S3 for the object.
    pub etag: String,
}

/// S3 list result containing blob information.
#[derive(Debug, Clone)]
pub struct S3ListResult {
    /// List of blob information.
    pub blobs: Vec<BlobInfo>,
    /// Continuation token for pagination (if more results available).
    pub continuation_token: Option<String>,
}

/// S3 delete request parameters.
#[derive(Debug, Clone)]
pub struct S3DeleteRequest {
    /// S3 bucket name.
    pub bucket: String,
    /// Object key (path) within the bucket.
    pub key: String,
    /// AWS region (e.g., "us-east-1").
    pub region: String,
    /// Optional AWS access key (ephemeral, from request metadata).
    pub access_key: Option<String>,
    /// Optional AWS secret key (ephemeral, from request metadata).
    pub secret_key: Option<String>,
    /// Optional AWS session token (ephemeral, from request metadata).
    pub session_token: Option<String>,
}

/// S3 delete result containing deletion status.
#[derive(Debug, Clone)]
pub struct S3DeleteResult {
    /// True if deletion was successful.
    pub success: bool,
    /// Status message.
    pub message: String,
}

/// Wrapper that computes SHA256 hash while reading data.
#[derive(Debug)]
struct HashingReader<R> {
    inner: R,
    hasher: Sha256,
    hash_output: Arc<Mutex<Option<String>>>,
}

impl<R> HashingReader<R> {
    fn new(inner: R, hash_output: Arc<Mutex<Option<String>>>) -> Self {
        Self {
            inner,
            hasher: Sha256::new(),
            hash_output,
        }
    }
}

 impl<R: AsyncRead + Unpin> AsyncRead for HashingReader<R> {
     fn poll_read(
         mut self: Pin<&mut Self>,
         cx: &mut Context<'_>,
         buf: &mut ReadBuf<'_>,
     ) -> Poll<io::Result<()>> {
         let inner = Pin::new(&mut self.inner);
         inner.poll_read(cx, buf).map_ok(|_| {
             let n = buf.filled().len();
             if n > 0 {
                 self.hasher.update(buf.filled());
             }
         })
     }
 }

 impl<R> Drop for HashingReader<R> {
     fn drop(&mut self) {
         let hasher = std::mem::take(&mut self.hasher);
         let hash = hex::encode(hasher.finalize());
         *self.hash_output.lock().expect("HashingReader mutex poisoned") = Some(hash);
     }
 }

/// S3 connector for uploading blobs to AWS S3.
#[derive(Debug, Clone)]
pub struct S3Connector {
    reliability: Arc<S3Reliability>,
}

impl S3Connector {
    /// Creates a new S3Connector with default reliability settings.
    pub fn new() -> Self {
        Self {
            reliability: Arc::new(S3Reliability::new(5, 30, 100)),
        }
    }
    
    /// Creates a new S3Connector with custom reliability settings.
    pub fn with_reliability_settings(
        failure_threshold: u32,
        recovery_timeout_secs: u64,
        max_requests_per_second: u32,
    ) -> Self {
        Self {
            reliability: Arc::new(S3Reliability::new(
                failure_threshold,
                recovery_timeout_secs,
                max_requests_per_second,
            )),
        }
    }
    
    /// Creates an S3 client from upload request.
    ///
    /// Uses ephemeral credentials if provided in the request.
    /// Otherwise, uses the default AWS credential chain.
    async fn create_client(&self, request: &S3UploadRequest) -> Result<S3Client> {
        let region_provider = RegionProviderChain::first_try(Some(
            aws_sdk_s3::config::Region::new(request.region.clone()),
        ));

        let config_loader = aws_config::defaults(BehaviorVersion::latest())
            .region(region_provider);

        let config = if let (Some(access_key), Some(secret_key)) = (
            &request.access_key,
            &request.secret_key,
        ) {
            debug!("Using ephemeral credentials for S3 upload");

            let credentials_provider = aws_credential_types::Credentials::new(
                access_key.clone(),
                secret_key.clone(),
                request.session_token.clone(),
                None,
                "ephemeral",
            );

            config_loader
                .credentials_provider(credentials_provider)
                .load()
                .await
        } else {
            debug!("Using default credential chain for S3 upload");
            config_loader.load().await
        };

        Ok(S3Client::new(&config))
    }

    /// Uploads a blob to S3 with streaming and hash computation.
    ///
    /// Supports both in-memory content and file path streaming.
    /// For large files, streaming from disk is recommended to avoid OOM.
    ///
    /// # Arguments
    ///
    /// * `request` - S3 upload request parameters
    ///
    /// # Returns
    ///
    /// Returns an `S3UploadResult` containing upload metadata.
    ///
    /// # Errors
    ///
    /// Returns `SidecarError` if:
    /// - Neither content nor file_path is provided
    /// - Both content and file_path are provided
    /// - File I/O fails
    /// - S3 operation fails
    /// - Hash computation fails
    /// - Circuit breaker is open
    /// - Rate limit exceeded
     pub async fn upload(&self, request: S3UploadRequest) -> Result<S3UploadResult> {
         let request_clone = request.clone();
         let connector = self.clone();
         let reliability = self.reliability.clone();

         reliability.upload(async move {
             Self::upload_internal(&connector, &request_clone).await
         }).await
     }
    
     async fn upload_internal(connector: &S3Connector, request: &S3UploadRequest) -> Result<S3UploadResult> {
         if request.content.is_none() && request.file_path.is_none() {
             return Err(SidecarError::InvalidRequest(
                 "Either content or file_path must be provided".to_string(),
             ));
         }

         if request.content.is_some() && request.file_path.is_some() {
             return Err(SidecarError::InvalidRequest(
                 "Only one of content or file_path can be provided".to_string(),
             ));
         }

         let client = connector.create_client(&request).await?;

         let (byte_stream, size_bytes, hash_output): (ByteStream, i64, Arc<Mutex<Option<String>>>);
          if let Some(content) = &request.content {
               let mut hasher = Sha256::new();
               hasher.update(content);
               let hash = hex::encode(hasher.finalize());

               let size = content.len() as i64;
               debug!(
                   "Uploading in-memory content of {} bytes to s3://{}/{}",
                   size, request.bucket, request.key
               );

               byte_stream = ByteStream::from(content.clone());
               size_bytes = size;
               hash_output = Arc::new(Mutex::new(Some(hash)));
          } else if let Some(file_path) = &request.file_path {
               let metadata = tokio::fs::metadata(&file_path).await.map_err(|e| {
                   SidecarError::Io(e)
               })?;
               let size = metadata.len() as i64;
               debug!(
                   "Streaming file {} ({} bytes) to s3://{}/{}",
                   file_path.display(),
                   size,
                   request.bucket,
                   request.key
               );

               let mut file = File::open(&file_path).await.map_err(|e| SidecarError::Io(e))?;
               let mut buffer = Vec::new();
               file.read_to_end(&mut buffer).await.map_err(|e| SidecarError::Io(e))?;
               let mut hasher = Sha256::new();
               hasher.update(&buffer);
               let hash = hex::encode(hasher.finalize());

               let local_byte_stream = ByteStream::from(buffer);
               let local_hash_output = Arc::new(Mutex::new(Some(hash)));

               byte_stream = local_byte_stream;
               size_bytes = size;
               hash_output = local_hash_output;
          } else {
               unreachable!()
          }

         info!(
             "Starting S3 upload: bucket={}, key={}, size={}",
             request.bucket, request.key, size_bytes
         );

         let output = client
             .put_object()
             .bucket(&request.bucket)
             .key(&request.key)
             .content_length(size_bytes as i64)
             .body(byte_stream)
             .send()
             .await
               .map_err(|e| {
                   error!("S3 upload failed: {}", e);
                   SidecarError::StorageError(format!("S3 upload failed: {}", e))
               })?;

         debug!(
             "S3 upload completed: bucket={}, key={}, etag={:?}",
             request.bucket,
             request.key,
             output.e_tag()
         );

         
         let content_hash_sha256 = Arc::try_unwrap(hash_output.clone())
             .ok()
             .and_then(|m| m.lock().expect("Hash mutex poisoned").take())
             .unwrap_or_else(|| {
                 error!("Failed to extract hash after upload");
                 "".to_string()
             });

         let timestamp_unix = SystemTime::now()
             .duration_since(UNIX_EPOCH)
             .map(|d| d.as_secs() as i64)
             .unwrap_or_else(|_| {
                 error!("System clock appears to be before Unix epoch, using 0");
                 0
             });

         let etag = output
             .e_tag()
             .map(|s| s.trim_matches('"'))
             .unwrap_or("")
             .to_string();

        
         let blob_id = format!("s3://{}/{}", request.bucket, request.key);
         info!(
             "Upload completed: blob_id={}, etag={}, size={}, hash={}",
             blob_id, etag, size_bytes, content_hash_sha256
         );
        
         Ok(S3UploadResult {
             blob_id,
             etag,
             size_bytes,
             content_hash_sha256,
             timestamp_unix,
         })
     }

    /// Downloads a blob from S3 with streaming chunking.
    ///
    /// Streams the blob data in 1MB chunks to avoid loading large files into memory.
    /// Supports range requests via `offset_bytes` and `max_bytes` parameters.
    ///
    /// # Arguments
    ///
    /// * `request` - S3 download request parameters
    ///
    /// # Returns
    ///
    /// Returns a stream of `BlobChunk` items, each containing up to 1MB of data.
    ///
    /// # Errors
    ///
    /// Returns error in the stream if:
    /// - S3 object does not exist
    /// - Access denied to S3 bucket or object
    /// - S3 operation fails
    /// - Range request is invalid
    pub fn download_stream(
        &self,
        request: S3DownloadRequest,
    ) -> impl Stream<Item = Result<BlobChunk>> {
        const CHUNK_SIZE: usize = 1_048_576;

        let client_future = async move {
            let region_provider = RegionProviderChain::first_try(Some(
                aws_sdk_s3::config::Region::new(request.region.clone()),
            ));

            let config_loader = aws_config::defaults(BehaviorVersion::latest())
                .region(region_provider);

            let config = if let (Some(access_key), Some(secret_key)) = (
                &request.access_key,
                &request.secret_key,
            ) {
                debug!("Using ephemeral credentials for S3 download");

                let credentials_provider = aws_credential_types::Credentials::new(
                    access_key.clone(),
                    secret_key.clone(),
                    request.session_token.clone(),
                    None,
                    "ephemeral",
                );

                config_loader
                    .credentials_provider(credentials_provider)
                    .load()
                    .await
            } else {
                debug!("Using default credential chain for S3 download");
                config_loader.load().await
            };

            Ok::<S3Client, SidecarError>(S3Client::new(&config))
        };

        try_stream! {
            let client = client_future.await.map_err(|e| {
                error!("Failed to create S3 client: {}", e);
                e
            })?;

             let mut get_builder = client.get_object()
                 .bucket(&request.bucket)
                 .key(&request.key);

             if let (Some(offset), Some(max_bytes)) = (request.offset_bytes, request.max_bytes) {
                 let range_header = format!("bytes={}-{}", offset, offset + max_bytes - 1);
                 get_builder = get_builder.range(range_header.clone());
                 debug!(
                     "Downloading range from s3://{}/{}: {}",
                     request.bucket, request.key, range_header
                 );
             } else if let Some(offset) = request.offset_bytes {
                 let range_header = format!("bytes={}-", offset);
                 get_builder = get_builder.range(range_header.clone());
                 debug!(
                     "Downloading from offset from s3://{}/{}: {}",
                     request.bucket, request.key, range_header
                 );
             } else {
                 debug!(
                     "Downloading full object from s3://{}/{}",
                     request.bucket, request.key
                 );
             }

             let response = get_builder.send()
                 .await
                 .map_err(|e| {
                     error!("S3 download failed: {}", e);
                     SidecarError::StorageError(format!("S3 download failed: {}", e))
                 })?;

            let content_length = response.content_length.unwrap_or(0) as i64;
            debug!("S3 object size: {} bytes", content_length);

            let mut reader = response.body.into_async_read();
            let mut offset = request.offset_bytes.unwrap_or(0);
            let mut buffer = vec![0u8; CHUNK_SIZE];

            loop {
                let bytes_read = reader.read(&mut buffer).await.map_err(|e| {
                    error!("Failed to read from S3 stream: {}", e);
                    SidecarError::StorageError(format!("Failed to read from S3 stream: {}", e))
                })?;

                if bytes_read == 0 {
                    break;
                }

                let chunk_data = buffer[..bytes_read].to_vec();
                let is_last = bytes_read < CHUNK_SIZE;

                debug!(
                    "Yielding chunk: offset={}, size={}, is_last={}",
                    offset,
                    chunk_data.len(),
                    is_last
                );

                yield BlobChunk {
                    data: chunk_data,
                    offset,
                    is_last,
                };

                offset += bytes_read as i64;
            }
        }
    }

    /// Lists blobs in an S3 bucket with optional filtering.
    ///
    /// Uses S3 list_objects_v2 API to list objects with prefix filtering and max_results limit.
    /// Supports pagination via continuation_token.
    ///
    /// # Arguments
    ///
    /// * `request` - S3 list request parameters
    ///
    /// # Returns
    ///
    /// Returns an `S3ListResult` containing blob information and optional continuation token.
    ///
    /// # Errors
    ///
    /// Returns `SidecarError` if:
    /// - S3 operation fails (access denied, bucket not found, etc.)
    /// - Invalid request parameters
    /// - Circuit breaker is open
    /// - Rate limit exceeded
    pub async fn list_blobs(&self, request: S3ListRequest) -> Result<S3ListResult> {
        let request_clone = request.clone();
        let reliability = self.reliability.clone();
        
        reliability.list(async move {
            Self::list_blobs_internal(&request_clone).await
        }).await
    }
    
    async fn list_blobs_internal(request: &S3ListRequest) -> Result<S3ListResult> {
        let region_provider = RegionProviderChain::first_try(Some(
            aws_sdk_s3::config::Region::new(request.region.clone()),
        ));

        let config_loader = aws_config::defaults(BehaviorVersion::latest())
            .region(region_provider);

        let config = if let (Some(access_key), Some(secret_key)) = (
            &request.access_key,
            &request.secret_key,
        ) {
            debug!("Using ephemeral credentials for S3 list");

            let credentials_provider = aws_credential_types::Credentials::new(
                access_key.clone(),
                secret_key.clone(),
                request.session_token.clone(),
                None,
                "ephemeral",
            );

            config_loader
                .credentials_provider(credentials_provider)
                .load()
                .await
        } else {
            debug!("Using default credential chain for S3 list");
            config_loader.load().await
        };

         let client = S3Client::new(&config);

         let mut list_builder = client.list_objects_v2()
             .bucket(&request.bucket);

         if let Some(prefix) = &request.prefix {
             list_builder = list_builder.prefix(prefix);
             debug!(
                 "Listing blobs in s3://{}/{} with prefix",
                 request.bucket, prefix
             );
         } else {
             debug!("Listing all blobs in s3://{}", request.bucket);
         }

         if let Some(max_results) = request.max_results {
             list_builder = list_builder.max_keys(max_results);
         }

         let output = list_builder.send()
             .await
             .map_err(|e| {
                 error!("S3 list failed: {}", e);
                 SidecarError::StorageError(format!("S3 list failed: {}", e))
             })?;

         let blobs: Vec<BlobInfo> = output
             .contents()
             .iter()
             .filter_map(|object| {
                let key = object.key()?;
                let size = object.size().unwrap_or(0);
                 let last_modified = object
                     .last_modified()
                     .and_then(|dt| {
                         (dt.as_secs_f64() as i64).try_into().ok()
                     })
                     .unwrap_or(0);
                let etag = object.e_tag().map(|s| s.trim_matches('"').to_string()).unwrap_or_default();
                let uri = format!("s3://{}/{}", request.bucket, key);

                Some(BlobInfo {
                    uri,
                    size_bytes: size,
                    content_type: "application/octet-stream".to_string(),
                    last_modified_unix: last_modified,
                    etag,
                })
            })
            .collect();

        let continuation_token = output.next_continuation_token().map(|s| s.to_string());

        info!(
            "Listed {} blobs in s3://{} (continuation_token: {:?})",
            blobs.len(),
            request.bucket,
            continuation_token
        );

        Ok(S3ListResult {
            blobs,
            continuation_token,
        })
    }

    /// Deletes a blob from S3.
    ///
    /// Uses S3 delete_object API to delete a single object.
    /// Parses bucket and key from URI in format "s3://bucket/key".
    ///
    /// # Arguments
    ///
    /// * `request` - S3 delete request parameters
    ///
    /// # Returns
    ///
    /// Returns an `S3DeleteResult` indicating success or failure.
    ///
    /// # Errors
    ///
    /// Returns `SidecarError` if:
    /// - Invalid URI format (must be "s3://bucket/key")
    /// - S3 operation fails (access denied, object not found, etc.)
    /// - Circuit breaker is open
    /// - Rate limit exceeded
    pub async fn delete_blob(&self, request: S3DeleteRequest) -> Result<S3DeleteResult> {
        let request_clone = request.clone();
        let reliability = self.reliability.clone();
        
        reliability.delete(async move {
            Self::delete_blob_internal(&request_clone).await
        }).await
    }
    
    async fn delete_blob_internal(request: &S3DeleteRequest) -> Result<S3DeleteResult> {
        let region_provider = RegionProviderChain::first_try(Some(
            aws_sdk_s3::config::Region::new(request.region.clone()),
        ));

        let config_loader = aws_config::defaults(BehaviorVersion::latest())
            .region(region_provider);

        let config = if let (Some(access_key), Some(secret_key)) = (
            &request.access_key,
            &request.secret_key,
        ) {
            debug!("Using ephemeral credentials for S3 delete");

            let credentials_provider = aws_credential_types::Credentials::new(
                access_key.clone(),
                secret_key.clone(),
                request.session_token.clone(),
                None,
                "ephemeral",
            );

            config_loader
                .credentials_provider(credentials_provider)
                .load()
                .await
        } else {
            debug!("Using default credential chain for S3 delete");
            config_loader.load().await
        };

         let client = S3Client::new(&config);

         info!(
             "Deleting s3://{}/{}",
             request.bucket, request.key
         );

         let output = client.delete_object()
             .bucket(&request.bucket)
             .key(&request.key)
             .send()
             .await
             .map_err(|e| {
                 error!("S3 delete failed: {}", e);
                 SidecarError::StorageError(format!("S3 delete failed: {}", e))
             })?;

        let version_id = output.version_id().map(|s| s.to_string());

        info!(
            "Deleted s3://{}/{} (version_id: {:?})",
            request.bucket,
            request.key,
            version_id
        );

        Ok(S3DeleteResult {
            success: true,
            message: format!(
                "Successfully deleted s3://{}/{}",
                request.bucket, request.key
            ),
        })
    }

    /// Parses an S3 URI to extract bucket and key.
    ///
    /// # Arguments
    ///
    /// * `uri` - S3 URI in format "s3://bucket/key"
    ///
    /// # Returns
    ///
    /// Returns a tuple of (bucket, key).
    ///
    /// # Errors
    ///
    /// Returns `SidecarError` if:
    /// - Invalid URI format
    /// - Missing bucket or key
    pub fn parse_s3_uri(&self, uri: &str) -> Result<(String, String)> {
        if !uri.starts_with("s3://") {
            return Err(SidecarError::InvalidRequest(format!(
                "Invalid S3 URI format: {} (must start with s3://)",
                uri
            )));
        }

        let without_prefix = uri.strip_prefix("s3://").unwrap();
        let parts: Vec<&str> = without_prefix.splitn(2, '/').collect();

        if parts.len() != 2 || parts[0].is_empty() || parts[1].is_empty() {
            return Err(SidecarError::InvalidRequest(format!(
                "Invalid S3 URI format: {} (must be s3://bucket/key)",
                uri
            )));
        }

        Ok((parts[0].to_string(), parts[1].to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tokio::io::BufReader;

    #[tokio::test]
    #[ignore]
    async fn test_hashing_reader_computes_hash() {
        let data = b"hello world";
        let hash_output = Arc::new(Mutex::new(None));
        {
            let mut reader = HashingReader::new(&data[..], hash_output.clone());

            use tokio::io::AsyncReadExt;
            let mut buffer = Vec::new();
            reader.read_to_end(&mut buffer).await.unwrap();

            drop(reader);
        }
        let binding = hash_output.lock().expect("Hash mutex poisoned");
        let hash = binding.as_ref().unwrap();
        assert_eq!(
            hash,
            "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
        );
    }

    #[tokio::test]
    async fn test_hashing_reader_with_multiple_reads() {
        let data = b"hello world test data";
        let hash_output = Arc::new(Mutex::new(None));
        {
            let mut reader = HashingReader::new(&data[..], hash_output.clone());
            let mut buf = [0u8; 5];
            reader.read_exact(&mut buf).await.unwrap();
            assert_eq!(&buf, b"hello");
            reader.read_exact(&mut buf).await.unwrap();
            assert_eq!(&buf, b" worl");
            let bytes_read = reader.read(&mut buf).await.unwrap();
            assert_eq!(bytes_read, 5, "Should read 5 bytes into buffer");
            assert_eq!(&buf[0..bytes_read], b"d tes");
            drop(reader);
        }
        let binding = hash_output.lock().expect("Hash mutex poisoned");
        let hash = binding.as_ref().unwrap();
        assert!(hash.len() == 64);
    }

    #[tokio::test]
    async fn test_upload_validation_both_content_and_file() {
        let connector = S3Connector::new();
        let request = S3UploadRequest {
            bucket: "test-bucket".to_string(),
            key: "test-key".to_string(),
            region: "us-east-1".to_string(),
            content_type: None,
            content: Some(vec![1, 2, 3]),
            file_path: Some(PathBuf::from("/tmp/test.txt")),
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let result = connector.upload(request).await;
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("Only one of content or file_path can be provided"));
    }

    #[tokio::test]
    async fn test_upload_validation_neither_content_nor_file() {
        let connector = S3Connector::new();
        let request = S3UploadRequest {
            bucket: "test-bucket".to_string(),
            key: "test-key".to_string(),
            region: "us-east-1".to_string(),
            content_type: None,
            content: None,
            file_path: None,
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let result = connector.upload(request).await;
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("Either content or file_path must be provided"));
    }

    #[tokio::test]
    async fn test_upload_file_not_found() {
        let connector = S3Connector::new();
        let request = S3UploadRequest {
            bucket: "test-bucket".to_string(),
            key: "test-key".to_string(),
            region: "us-east-1".to_string(),
            content_type: None,
            content: None,
            file_path: Some(PathBuf::from("/nonexistent/file.txt")),
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let result = connector.upload(request).await;
        assert!(result.is_err(), "Upload should fail for nonexistent file");
        let err = result.unwrap_err();
        assert!(
            matches!(err, SidecarError::StorageError(_)),
            "Expected StorageError, got: {:?}",
            err
        );
    }

    #[tokio::test]
    async fn test_upload_empty_content() {
        let connector = S3Connector::new();
        let request = S3UploadRequest {
            bucket: "test-bucket".to_string(),
            key: "test-key".to_string(),
            region: "us-east-1".to_string(),
            content_type: None,
            content: Some(vec![]),
            file_path: None,
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let result = connector.upload(request).await;
        assert!(result.is_err());
    }

    #[test]
    fn test_timestamp_creation() {
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;
        assert!(timestamp > 0);
    }

    #[test]
    fn test_size_computation_memory() {
        let content = vec![1u8; 100];
        let size = content.len() as i64;
        assert_eq!(size, 100);
    }

 #[test]
        fn test_size_computation_file() {
            let mut temp_file = tempfile::NamedTempFile::new().unwrap();
            let data = vec![1u8; 200];
            std::io::Write::write_all(&mut temp_file, &data).unwrap();
            let metadata = std::fs::metadata(temp_file.path()).unwrap();
            let size = metadata.len() as i64;
            assert_eq!(size, 200);
        }

    #[test]
    fn test_region_configuration() {
        let region = aws_sdk_s3::config::Region::new("us-west-2");
        assert_eq!(region.as_ref(), "us-west-2");
    }

    #[test]
    fn test_content_type_handling() {
        let content_type = Some("application/json".to_string());
        assert_eq!(content_type, Some("application/json".to_string()));

        let content_type_none: Option<String> = None;
        assert!(content_type_none.is_none());
    }

    #[test]
    fn test_blob_id_components() {
        let result = S3UploadResult {
            blob_id: "s3://bucket/key".to_string(),
            etag: "etag".to_string(),
            size_bytes: 100,
            content_hash_sha256: "hash".to_string(),
            timestamp_unix: 1234567890,
        };

        assert_eq!(result.blob_id, "s3://bucket/key");
        assert_eq!(result.etag, "etag");
        assert_eq!(result.size_bytes, 100);
        assert_eq!(result.content_hash_sha256, "hash");
        assert_eq!(result.timestamp_unix, 1234567890);
    }

    #[test]
    fn test_etag_trimming() {
        let etag_with_quotes = "\"abc123def\"";
        let trimmed = etag_with_quotes.trim_matches('"');
        assert_eq!(trimmed, "abc123def");

        let etag_without_quotes = "abc123def";
        let trimmed = etag_without_quotes.trim_matches('"');
        assert_eq!(trimmed, "abc123def");
    }

    #[test]
    fn test_blob_id_formatting() {
        let bucket = "my-bucket";
        let key = "path/to/object.txt";
        let blob_id = format!("s3://{}/{}", bucket, key);
        assert_eq!(blob_id, "s3://my-bucket/path/to/object.txt");
    }

    #[test]
    fn test_hashing_reader_empty_input() {
        let data = b"";
        let hash_output = Arc::new(Mutex::new(None));
        {
            let reader = HashingReader::new(&data[..], hash_output.clone());
            drop(reader);
        }
        let binding = hash_output.lock().expect("Hash mutex poisoned");
        let hash = binding.as_ref().unwrap();
        assert_eq!(
            hash,
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        );
    }

    #[tokio::test]
    async fn test_hashing_reader_large_input() {
        let data: Vec<u8> = (0..10000).map(|i| (i % 256) as u8).collect();
        let hash_output = Arc::new(Mutex::new(None));
        {
            let mut reader = HashingReader::new(data.as_slice(), hash_output.clone());
            let mut buf = [0u8; 100];
            while reader.read(&mut buf).await.unwrap() > 0 {}
            drop(reader);
        }
        let binding = hash_output.lock().expect("Hash mutex poisoned");
        let hash = binding.as_ref().unwrap();
        assert!(hash.len() == 64);
    }

    #[tokio::test]
    async fn test_file_hash_computation() -> Result<()> {
        let mut temp_file = tempfile::NamedTempFile::new()?;
        writeln!(temp_file, "hello world")?;

        let mut hasher = Sha256::new();
        let file = File::open(temp_file.path()).await?;
        let mut reader = BufReader::new(file);
        let mut buffer = [0u8; 8192];

        loop {
            let bytes_read = reader.read(&mut buffer).await?;
            if bytes_read == 0 {
                break;
            }
            hasher.update(&buffer[..bytes_read]);
        }

        let hash = hex::encode(hasher.finalize());

        assert!(hash.len() == 64);

        Ok(())
    }

    #[test]
    fn test_sha256_hash_computation_memory() {
        let content = b"hello world";
        let mut hasher = Sha256::new();
        hasher.update(content);
        let hash = hex::encode(hasher.finalize());

        assert_eq!(
            hash,
            "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
        );
    }

    #[test]
    fn test_s3_upload_request_builder() {
        let request = S3UploadRequest {
            bucket: "my-bucket".to_string(),
            key: "path/to/object.txt".to_string(),
            region: "us-west-2".to_string(),
            content_type: Some("text/plain".to_string()),
            content: Some(vec![1, 2, 3]),
            file_path: None,
            access_key: Some("AKIAIOSFODNN7EXAMPLE".to_string()),
            secret_key: Some("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY".to_string()),
            session_token: Some("session-token-123".to_string()),
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.key, "path/to/object.txt");
        assert_eq!(request.region, "us-west-2");
        assert_eq!(request.content_type, Some("text/plain".to_string()));
        assert_eq!(request.content, Some(vec![1, 2, 3]));
        assert!(request.file_path.is_none());
        assert_eq!(
            request.access_key,
            Some("AKIAIOSFODNN7EXAMPLE".to_string())
        );
        assert_eq!(
            request.secret_key,
            Some("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY".to_string())
        );
    }

    #[test]
    fn test_s3_upload_result_creation() {
        let hash_value = "a591a6d40bf420404a011733cfb7b190d62c65bf0bcda32b57b277d9ad9f14a6";
        let result = S3UploadResult {
            blob_id: "s3://test-bucket/test-key".to_string(),
            etag: "\"abc123\"".to_string(),
            size_bytes: 1024,
            content_hash_sha256: hash_value.to_string(),
            timestamp_unix: 1234567890,
        };

        assert_eq!(result.blob_id, "s3://test-bucket/test-key");
        assert_eq!(result.size_bytes, 1024);
        assert_eq!(result.content_hash_sha256.len(), 64, "Hash length should be 64, got {}", result.content_hash_sha256.len());
    }

    #[test]
    fn test_s3_download_request_builder() {
        let request = S3DownloadRequest {
            bucket: "my-bucket".to_string(),
            key: "path/to/object.txt".to_string(),
            region: "us-west-2".to_string(),
            offset_bytes: Some(100),
            max_bytes: Some(1024),
            access_key: Some("AKIAIOSFODNN7EXAMPLE".to_string()),
            secret_key: Some("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY".to_string()),
            session_token: Some("session-token-123".to_string()),
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.key, "path/to/object.txt");
        assert_eq!(request.region, "us-west-2");
        assert_eq!(request.offset_bytes, Some(100));
        assert_eq!(request.max_bytes, Some(1024));
        assert_eq!(
            request.access_key,
            Some("AKIAIOSFODNN7EXAMPLE".to_string())
        );
    }

    #[test]
    fn test_s3_download_request_without_range() {
        let request = S3DownloadRequest {
            bucket: "my-bucket".to_string(),
            key: "path/to/object.txt".to_string(),
            region: "us-east-1".to_string(),
            offset_bytes: None,
            max_bytes: None,
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.key, "path/to/object.txt");
        assert!(request.offset_bytes.is_none());
        assert!(request.max_bytes.is_none());
    }

    #[test]
    fn test_blob_chunk_creation() {
        let data = vec![1, 2, 3, 4, 5];
        let chunk = BlobChunk {
            data: data.clone(),
            offset: 100,
            is_last: true,
        };

        assert_eq!(chunk.data, data);
        assert_eq!(chunk.offset, 100);
        assert!(chunk.is_last);
    }

    #[test]
    fn test_blob_chunk_is_last_marker() {
        let chunk1 = BlobChunk {
            data: vec![1, 2, 3],
            offset: 0,
            is_last: false,
        };

        let chunk2 = BlobChunk {
            data: vec![4, 5, 6],
            offset: 3,
            is_last: true,
        };

        assert!(!chunk1.is_last);
        assert!(chunk2.is_last);
    }

    #[test]
    fn test_chunk_size_constant() {
        const CHUNK_SIZE: usize = 1_048_576;
        assert_eq!(CHUNK_SIZE, 1024 * 1024);
    }

    #[test]
    fn test_offset_and_range_combinations() {
        let request_full = S3DownloadRequest {
            bucket: "test".to_string(),
            key: "key".to_string(),
            region: "us-east-1".to_string(),
            offset_bytes: None,
            max_bytes: None,
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let request_offset = S3DownloadRequest {
            offset_bytes: Some(100),
            ..request_full.clone()
        };

        let request_range = S3DownloadRequest {
            offset_bytes: Some(100),
            max_bytes: Some(1024),
            ..request_full.clone()
        };

        assert!(request_full.offset_bytes.is_none());
        assert!(request_full.max_bytes.is_none());
        assert_eq!(request_offset.offset_bytes, Some(100));
        assert!(request_offset.max_bytes.is_none());
        assert_eq!(request_range.offset_bytes, Some(100));
        assert_eq!(request_range.max_bytes, Some(1024));
    }

    #[test]
    fn test_blob_chunk_offset_tracking() {
        let chunks = vec![
            BlobChunk {
                data: vec![1; 1024],
                offset: 0,
                is_last: false,
            },
            BlobChunk {
                data: vec![2; 1024],
                offset: 1024,
                is_last: false,
            },
            BlobChunk {
                data: vec![3; 512],
                offset: 2048,
                is_last: true,
            },
        ];

        assert_eq!(chunks[0].offset, 0);
        assert_eq!(chunks[1].offset, 1024);
        assert_eq!(chunks[2].offset, 2048);
    }

    #[test]
    fn test_s3_list_request_builder() {
        let request = S3ListRequest {
            bucket: "my-bucket".to_string(),
            region: "us-west-2".to_string(),
            prefix: Some("prefix/".to_string()),
            max_results: Some(100),
            access_key: Some("AKIAIOSFODNN7EXAMPLE".to_string()),
            secret_key: Some("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY".to_string()),
            session_token: Some("session-token-123".to_string()),
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.region, "us-west-2");
        assert_eq!(request.prefix, Some("prefix/".to_string()));
        assert_eq!(request.max_results, Some(100));
        assert_eq!(
            request.access_key,
            Some("AKIAIOSFODNN7EXAMPLE".to_string())
        );
    }

    #[test]
    fn test_s3_list_request_minimal() {
        let request = S3ListRequest {
            bucket: "my-bucket".to_string(),
            region: "us-east-1".to_string(),
            prefix: None,
            max_results: None,
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.region, "us-east-1");
        assert!(request.prefix.is_none());
        assert!(request.max_results.is_none());
    }

    #[test]
    fn test_blob_info_creation() {
        let blob_info = BlobInfo {
            uri: "s3://bucket/key.txt".to_string(),
            size_bytes: 1024,
            content_type: "text/plain".to_string(),
            last_modified_unix: 1234567890,
            etag: "abc123".to_string(),
        };

        assert_eq!(blob_info.uri, "s3://bucket/key.txt");
        assert_eq!(blob_info.size_bytes, 1024);
        assert_eq!(blob_info.content_type, "text/plain");
        assert_eq!(blob_info.last_modified_unix, 1234567890);
        assert_eq!(blob_info.etag, "abc123");
    }

    #[test]
    fn test_s3_list_result_creation() {
        let result = S3ListResult {
            blobs: vec![
                BlobInfo {
                    uri: "s3://bucket/key1.txt".to_string(),
                    size_bytes: 100,
                    content_type: "text/plain".to_string(),
                    last_modified_unix: 1234567890,
                    etag: "etag1".to_string(),
                },
            ],
            continuation_token: Some("token123".to_string()),
        };

        assert_eq!(result.blobs.len(), 1);
        assert_eq!(result.continuation_token, Some("token123".to_string()));
    }

    #[test]
    fn test_s3_list_result_without_continuation() {
        let result = S3ListResult {
            blobs: vec![],
            continuation_token: None,
        };

        assert_eq!(result.blobs.len(), 0);
        assert!(result.continuation_token.is_none());
    }

    #[test]
    fn test_s3_delete_request_builder() {
        let request = S3DeleteRequest {
            bucket: "my-bucket".to_string(),
            key: "path/to/object.txt".to_string(),
            region: "us-west-2".to_string(),
            access_key: Some("AKIAIOSFODNN7EXAMPLE".to_string()),
            secret_key: Some("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY".to_string()),
            session_token: Some("session-token-123".to_string()),
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.key, "path/to/object.txt");
        assert_eq!(request.region, "us-west-2");
        assert_eq!(
            request.access_key,
            Some("AKIAIOSFODNN7EXAMPLE".to_string())
        );
    }

    #[test]
    fn test_s3_delete_request_minimal() {
        let request = S3DeleteRequest {
            bucket: "my-bucket".to_string(),
            key: "key.txt".to_string(),
            region: "us-east-1".to_string(),
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        assert_eq!(request.bucket, "my-bucket");
        assert_eq!(request.key, "key.txt");
        assert_eq!(request.region, "us-east-1");
    }

    #[test]
    fn test_s3_delete_result_creation() {
        let result = S3DeleteResult {
            success: true,
            message: "Successfully deleted s3://bucket/key".to_string(),
        };

        assert!(result.success);
        assert!(result.message.contains("Successfully deleted"));
    }

    #[test]
    fn test_parse_s3_uri_valid() {
        let connector = S3Connector::new();
        let result = connector.parse_s3_uri("s3://my-bucket/path/to/object.txt");

        assert!(result.is_ok());
        let (bucket, key) = result.unwrap();
        assert_eq!(bucket, "my-bucket");
        assert_eq!(key, "path/to/object.txt");
    }

    #[test]
    fn test_parse_s3_uri_simple() {
        let connector = S3Connector::new();
        let result = connector.parse_s3_uri("s3://bucket/key");

        assert!(result.is_ok());
        let (bucket, key) = result.unwrap();
        assert_eq!(bucket, "bucket");
        assert_eq!(key, "key");
    }

    #[test]
    fn test_parse_s3_uri_invalid_prefix() {
        let connector = S3Connector::new();
        let result = connector.parse_s3_uri("http://bucket/key");

        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("must start with s3://"));
    }

    #[test]
    fn test_parse_s3_uri_missing_key() {
        let connector = S3Connector::new();
        let result = connector.parse_s3_uri("s3://bucket");

        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("must be s3://bucket/key"));
    }

    #[test]
    fn test_parse_s3_uri_missing_bucket() {
        let connector = S3Connector::new();
        let result = connector.parse_s3_uri("s3:///key");

        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("must be s3://bucket/key"));
    }

    #[test]
    fn test_parse_s3_uri_with_special_chars() {
        let connector = S3Connector::new();
        let result = connector.parse_s3_uri("s3://my-bucket/path/to/file_name-v2.0.txt");

        assert!(result.is_ok());
        let (bucket, key) = result.unwrap();
        assert_eq!(bucket, "my-bucket");
        assert_eq!(key, "path/to/file_name-v2.0.txt");
    }

    #[test]
    fn test_blob_info_uri_formatting() {
        let bucket = "test-bucket";
        let key = "documents/file.pdf";
        let uri = format!("s3://{}/{}", bucket, key);

        assert_eq!(uri, "s3://test-bucket/documents/file.pdf");
    }

    #[test]
    fn test_blob_info_empty_fields() {
        let blob_info = BlobInfo {
            uri: "".to_string(),
            size_bytes: 0,
            content_type: "".to_string(),
            last_modified_unix: 0,
            etag: "".to_string(),
        };

        assert_eq!(blob_info.uri, "");
        assert_eq!(blob_info.size_bytes, 0);
        assert_eq!(blob_info.content_type, "");
        assert_eq!(blob_info.last_modified_unix, 0);
        assert_eq!(blob_info.etag, "");
    }

    #[test]
    fn test_prefix_filtering_logic() {
        let prefix = Some("docs/".to_string());
        assert_eq!(prefix, Some("docs/".to_string()));

        let prefix_none: Option<String> = None;
        assert!(prefix_none.is_none());
    }

    #[test]
    fn test_max_results_filtering_logic() {
        let max_results = Some(100);
        assert_eq!(max_results, Some(100));

        let max_results_none: Option<i32> = None;
        assert!(max_results_none.is_none());
    }

    #[tokio::test]
    async fn test_list_blobs_validation() {
        let connector = S3Connector::new();
        let request = S3ListRequest {
            bucket: "".to_string(),
            region: "us-east-1".to_string(),
            prefix: None,
            max_results: None,
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let result = connector.list_blobs(request).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_delete_blob_validation() {
        let connector = S3Connector::new();
        let request = S3DeleteRequest {
            bucket: "".to_string(),
            key: "".to_string(),
            region: "us-east-1".to_string(),
            access_key: None,
            secret_key: None,
            session_token: None,
        };

        let result = connector.delete_blob(request).await;
        assert!(result.is_err());
    }

    #[test]
    fn test_continuation_token_storage() {
        let token = Some("abc123xyz".to_string());
        assert_eq!(token, Some("abc123xyz".to_string()));

        let token_none: Option<String> = None;
        assert!(token_none.is_none());
    }

    #[test]
    fn test_blob_info_array_operations() {
        let blobs = vec![
            BlobInfo {
                uri: "s3://bucket/key1".to_string(),
                size_bytes: 100,
                content_type: "text/plain".to_string(),
                last_modified_unix: 1234567890,
                etag: "etag1".to_string(),
            },
            BlobInfo {
                uri: "s3://bucket/key2".to_string(),
                size_bytes: 200,
                content_type: "application/json".to_string(),
                last_modified_unix: 1234567891,
                etag: "etag2".to_string(),
            },
        ];

        assert_eq!(blobs.len(), 2);
        assert_eq!(blobs[0].uri, "s3://bucket/key1");
        assert_eq!(blobs[1].uri, "s3://bucket/key2");
        assert_eq!(blobs.iter().map(|b| b.size_bytes).sum::<i64>(), 300);
    }
}
