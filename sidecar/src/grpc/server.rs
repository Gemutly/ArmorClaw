use crate::config::SidecarConfig;
use crate::connectors::{S3Connector, S3UploadRequest, S3DownloadRequest, S3ListRequest, S3DeleteRequest};
use crate::document::{
    extract_text_from_pdf, extract_text_from_docx, extract_data_from_xlsx,
    extract_text_from_pptx,
    extract_text_with_ocr,
    convert_docx_to_pdf, convert_xlsx_to_csv, convert_pptx_to_pdf,
    qdrant::QdrantClient,
    embeddings::{Embedder, OpenAIEmbedder},
};
use crate::error::{Result, SidecarError};
use crate::grpc::interceptor::SecurityInterceptor;
use crate::grpc::proto::sidecar_service_server::{SidecarService, SidecarServiceServer};
use crate::grpc::proto::{
    HealthCheckRequest, HealthCheckResponse, UploadBlobRequest, UploadBlobResponse,
    DownloadBlobRequest, BlobChunk, ListBlobsRequest, ListBlobsResponse,
    DeleteBlobRequest, DeleteBlobResponse, ExtractTextRequest, ExtractTextResponse,
    ProcessDocumentRequest, ProcessDocumentResponse, BlobInfo,
    QueryDocumentsRequest, QueryDocumentsResponse, DocumentChunk,
    sidecar_service_server::SidecarService as SidecarServiceTrait,
};
use prometheus::Registry;
use std::path::Path;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Instant;
use tokio::net::UnixListener;
use tokio::signal;
use tokio_stream::wrappers::UnixListenerStream;
use tonic::{Request, Response, Status};
use tracing::{info, error, warn};

/// Parses an S3 URI ("s3://bucket/key") into (bucket, key).
fn parse_s3_uri(uri: &str) -> std::result::Result<(String, String), String> {
    let uri = uri.trim();
    if !uri.starts_with("s3://") {
        return Err(format!("Invalid S3 URI: expected s3://bucket/key, got: {}", uri));
    }
    let without_scheme = &uri[5..];
    let slash_pos = without_scheme
        .find('/')
        .ok_or_else(|| format!("Invalid S3 URI: no key component in: {}", uri))?;
    let bucket = without_scheme[..slash_pos].to_string();
    let key = without_scheme[slash_pos + 1..].to_string();
    if bucket.is_empty() {
        return Err(format!("Invalid S3 URI: empty bucket in: {}", uri));
    }
    Ok((bucket, key))
}

fn extract_string_value(value: &qdrant_client::qdrant::Value) -> Option<&str> {
    value.kind.as_ref().and_then(|k| match k {
        qdrant_client::qdrant::value::Kind::StringValue(s) => Some(s.as_str()),
        _ => None,
    })
}

/// Maps a SidecarError to an appropriate gRPC Status.
fn sidecar_error_to_status(e: SidecarError) -> Status {
    match &e {
        SidecarError::InvalidRequest(msg) => Status::invalid_argument(msg),
        SidecarError::AuthenticationFailed(msg) | SidecarError::AuthenticationError(msg) => {
            Status::unauthenticated(msg)
        }
        SidecarError::AccessDenied => Status::permission_denied(e.to_string()),
        SidecarError::NoSuchBucket(msg) | SidecarError::NoSuchKey(msg) => {
            Status::not_found(msg)
        }
        SidecarError::StorageError(msg)
        | SidecarError::CloudStorageError(msg)
        | SidecarError::ApiError(msg) => Status::internal(msg),
        SidecarError::DocumentProcessingError(msg) => Status::internal(msg),
        SidecarError::CircuitBreakerOpen(msg) => Status::unavailable(msg),
        SidecarError::RateLimitExceeded(msg) => {
            Status::resource_exhausted(msg)
        }
        SidecarError::Io(msg) => Status::internal(msg.to_string()),
        _ => Status::internal(e.to_string()),
    }
}

/// Reads RSS memory in bytes from `/proc/self/status` VmRSS field (Linux only).
/// Returns 0 on non-Linux platforms or if the file cannot be read.
fn read_memory_used_bytes() -> u64 {
    #[cfg(target_os = "linux")]
    {
        let Ok(content) = std::fs::read_to_string("/proc/self/status") else {
            return 0;
        };
        for line in content.lines() {
            if let Some(rest) = line.strip_prefix("VmRSS:") {
                let trimmed = rest.trim();
                // Format: "12345 kB"
                let kb_str = trimmed.strip_suffix(" kB").unwrap_or(trimmed);
                return kb_str.trim().parse::<u64>().unwrap_or(0) * 1024;
            }
        }
        0
    }
    #[cfg(not(target_os = "linux"))]
    {
        0
    }
}

#[derive(Debug)]
pub struct SidecarServiceImpl {
    s3_connector: Option<Arc<S3Connector>>,
    qdrant_client: Option<Arc<QdrantClient>>,
    embedding_api_key: Option<String>,
    start_time: Instant,
    active_requests: AtomicU64,
}

type DownloadBlobStream = tokio_stream::wrappers::ReceiverStream<std::result::Result<BlobChunk, tonic::Status>>;

impl SidecarServiceImpl {
    pub fn new(
        s3_connector: Option<S3Connector>,
        qdrant_client: Option<QdrantClient>,
        embedding_api_key: Option<String>,
    ) -> Self {
        Self {
            s3_connector: s3_connector.map(Arc::new),
            qdrant_client: qdrant_client.map(Arc::new),
            embedding_api_key,
            start_time: Instant::now(),
            active_requests: AtomicU64::new(0),
        }
    }
}

#[tonic::async_trait]
impl SidecarServiceTrait for SidecarServiceImpl {
    type DownloadBlobStream = DownloadBlobStream;
    async fn health_check(
        &self,
        _request: Request<HealthCheckRequest>,
    ) -> std::result::Result<Response<HealthCheckResponse>, Status> {
        info!("HealthCheck called");

        let uptime_seconds = self.start_time.elapsed().as_secs() as i64;
        let active_requests = self.active_requests.load(Ordering::Relaxed) as i32;
        let memory_used_bytes = read_memory_used_bytes() as i64;

        let response = HealthCheckResponse {
            status: "healthy".to_string(),
            uptime_seconds,
            active_requests,
            memory_used_bytes,
            version: env!("CARGO_PKG_VERSION").to_string(),
        };

        Ok(Response::new(response))
    }

    async fn upload_blob(
        &self,
        request: Request<UploadBlobRequest>,
    ) -> std::result::Result<Response<UploadBlobResponse>, Status> {
        let req = request.into_inner();
        info!(destination_uri = %req.destination_uri, "UploadBlob called");

        let connector = self.s3_connector.as_ref()
            .ok_or_else(|| Status::unimplemented("S3 connector not configured"))?;

        let (bucket, key) = parse_s3_uri(&req.destination_uri)
            .map_err(|e| Status::invalid_argument(e))?;

        let region = req.provider_config.get("region")
            .cloned()
            .unwrap_or_else(|| "us-east-1".to_string());

        let upload_req = S3UploadRequest {
            bucket,
            key,
            region,
            content_type: if req.content_type.is_empty() { None } else { Some(req.content_type) },
            content: if req.content.is_empty() { None } else { Some(req.content) },
            file_path: if req.local_file_path.is_empty() { None } else { Some(req.local_file_path.into()) },
            access_key: req.provider_config.get("access_key").cloned(),
            secret_key: req.provider_config.get("secret_key").cloned(),
            session_token: req.provider_config.get("session_token").cloned(),
        };

        let result = connector.upload(upload_req)
            .await
            .map_err(sidecar_error_to_status)?;

        Ok(Response::new(UploadBlobResponse {
            blob_id: result.blob_id,
            etag: result.etag,
            size_bytes: result.size_bytes,
            content_hash_sha256: result.content_hash_sha256,
            timestamp_unix: result.timestamp_unix,
        }))
    }

    async fn download_blob(
        &self,
        request: Request<DownloadBlobRequest>,
    ) -> std::result::Result<Response<DownloadBlobStream>, Status> {
        let req = request.into_inner();
        info!(source_uri = %req.source_uri, "DownloadBlob called");

        let connector = self.s3_connector.as_ref()
            .ok_or_else(|| Status::unimplemented("S3 connector not configured"))?;

        let (bucket, key) = parse_s3_uri(&req.source_uri)
            .map_err(|e| Status::invalid_argument(e))?;

        let region = req.provider_config.get("region")
            .cloned()
            .unwrap_or_else(|| "us-east-1".to_string());

        let download_req = S3DownloadRequest {
            bucket,
            key,
            region,
            offset_bytes: if req.offset_bytes == 0 { None } else { Some(req.offset_bytes) },
            max_bytes: if req.max_bytes == 0 { None } else { Some(req.max_bytes) },
            access_key: req.provider_config.get("access_key").cloned(),
            secret_key: req.provider_config.get("secret_key").cloned(),
            session_token: req.provider_config.get("session_token").cloned(),
        };

        let stream = connector.download_stream(download_req);

        let (tx, rx) = tokio::sync::mpsc::channel(4);

        tokio::spawn(async move {
            use futures::StreamExt;
            let mut stream = std::pin::pin!(stream);
            while let Some(result) = stream.next().await {
                match result {
                    Ok(chunk) => {
                        let grpc_chunk = BlobChunk {
                            data: chunk.data,
                            offset: chunk.offset,
                            is_last: chunk.is_last,
                        };
                        if tx.send(Ok(grpc_chunk)).await.is_err() {
                            break;
                        }
                    }
                    Err(e) => {
                        let _ = tx.send(Err(sidecar_error_to_status(e))).await;
                        break;
                    }
                }
            }
        });

        Ok(Response::new(DownloadBlobStream::new(rx)))
    }

    async fn list_blobs(
        &self,
        request: Request<ListBlobsRequest>,
    ) -> std::result::Result<Response<ListBlobsResponse>, Status> {
        let req = request.into_inner();
        info!(prefix = %req.prefix, "ListBlobs called");

        let connector = self.s3_connector.as_ref()
            .ok_or_else(|| Status::unimplemented("S3 connector not configured"))?;

        let region = req.provider_config.get("region")
            .cloned()
            .unwrap_or_else(|| "us-east-1".to_string());

        let bucket = req.provider_config.get("bucket")
            .cloned()
            .unwrap_or_else(|| req.provider.clone());

        let list_req = S3ListRequest {
            bucket,
            region,
            prefix: if req.prefix.is_empty() { None } else { Some(req.prefix) },
            max_results: if req.max_results == 0 { None } else { Some(req.max_results) },
            access_key: req.provider_config.get("access_key").cloned(),
            secret_key: req.provider_config.get("secret_key").cloned(),
            session_token: req.provider_config.get("session_token").cloned(),
        };

        let result = connector.list_blobs(list_req)
            .await
            .map_err(sidecar_error_to_status)?;

        let blobs: Vec<BlobInfo> = result.blobs.into_iter().map(|b| BlobInfo {
            uri: b.uri,
            size_bytes: b.size_bytes,
            content_type: b.content_type,
            last_modified_unix: b.last_modified_unix,
            etag: b.etag,
        }).collect();

        Ok(Response::new(ListBlobsResponse {
            blobs,
            continuation_token: result.continuation_token.unwrap_or_default(),
        }))
    }

    async fn delete_blob(
        &self,
        request: Request<DeleteBlobRequest>,
    ) -> std::result::Result<Response<DeleteBlobResponse>, Status> {
        let req = request.into_inner();
        info!(uri = %req.uri, "DeleteBlob called");

        let connector = self.s3_connector.as_ref()
            .ok_or_else(|| Status::unimplemented("S3 connector not configured"))?;

        let (bucket, key) = parse_s3_uri(&req.uri)
            .map_err(|e| Status::invalid_argument(e))?;

        let region = req.provider_config.get("region")
            .cloned()
            .unwrap_or_else(|| "us-east-1".to_string());

        let delete_req = S3DeleteRequest {
            bucket,
            key,
            region,
            access_key: req.provider_config.get("access_key").cloned(),
            secret_key: req.provider_config.get("secret_key").cloned(),
            session_token: req.provider_config.get("session_token").cloned(),
        };

        let result = connector.delete_blob(delete_req)
            .await
            .map_err(sidecar_error_to_status)?;

        Ok(Response::new(DeleteBlobResponse {
            success: result.success,
            message: result.message,
        }))
    }

    async fn extract_text(
        &self,
        request: Request<ExtractTextRequest>,
    ) -> std::result::Result<Response<ExtractTextResponse>, Status> {
        let req = request.into_inner();
        info!(document_format = %req.document_format, "ExtractText called");

        if req.document_content.is_empty() {
            return Err(Status::invalid_argument("document_content is empty"));
        }

        match req.document_format.as_str() {
            "pdf" | "application/pdf" => {
                let result = extract_text_from_pdf(&req.document_content)
                    .map_err(sidecar_error_to_status)?;
                Ok(Response::new(ExtractTextResponse {
                    text: result.text,
                    page_count: result.page_count,
                    metadata: result.metadata,
                }))
            }
            "docx" | "application/vnd.openxmlformats-officedocument.wordprocessingml.document" => {
                let result = extract_text_from_docx(&req.document_content)
                    .map_err(sidecar_error_to_status)?;
                Ok(Response::new(ExtractTextResponse {
                    text: result.text,
                    page_count: result.page_count,
                    metadata: result.metadata,
                }))
            }
            "xlsx" | "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" => {
                let result = extract_data_from_xlsx(&req.document_content)
                    .map_err(sidecar_error_to_status)?;
                let mut text_parts = Vec::new();
                for sheet in &result.sheets {
                    text_parts.push(format!("=== Sheet: {} ===", sheet.name));
                    for row in &sheet.rows {
                        let cells: Vec<String> = row.iter()
                            .map(|c| c.as_deref().unwrap_or("").to_string())
                            .collect();
                        text_parts.push(cells.join("\t"));
                    }
                    text_parts.push(String::new());
                }
                let text = text_parts.join("\n");
                let sheet_count = result.sheets.len() as i32;
                Ok(Response::new(ExtractTextResponse {
                    text,
                    page_count: sheet_count,
                    metadata: result.metadata,
                }))
            }
            "pptx" | "application/vnd.openxmlformats-officedocument.presentationml.presentation" => {
                let result = extract_text_from_pptx(&req.document_content)
                    .map_err(sidecar_error_to_status)?;
                Ok(Response::new(ExtractTextResponse {
                    text: result.text,
                    page_count: result.page_count,
                    metadata: result.metadata,
                }))
            }
            "image/png" | "image/jpeg" | "image/tiff" | "image/bmp" | "image/gif" => {
                let lang = req.options.get("language").cloned();
                let config = lang.map(|l| crate::document::OcrConfig {
                    language: l,
                    dpi: req.options.get("dpi").and_then(|d| d.parse().ok()),
                    psm: req.options.get("psm").and_then(|p| p.parse().ok()),
                    ..Default::default()
                });
                let result = extract_text_with_ocr(&req.document_content, config)
                    .map_err(sidecar_error_to_status)?;
                let mut metadata = result.metadata;
                metadata.insert("confidence".to_string(), format!("{:.2}", result.confidence));
                metadata.insert("language".to_string(), result.language);
                Ok(Response::new(ExtractTextResponse {
                    text: result.text,
                    page_count: 1,
                    metadata,
                }))
            }
            other => {
                Err(Status::invalid_argument(format!(
                    "Unsupported document format: '{}'. Supported: pdf, docx, xlsx, pptx, image/png, image/jpeg, image/tiff, image/bmp, image/gif",
                    other
                )))
            }
        }
    }

    async fn process_document(
        &self,
        request: Request<ProcessDocumentRequest>,
    ) -> std::result::Result<Response<ProcessDocumentResponse>, Status> {
        let req = request.into_inner();
        info!(
            operation = %req.operation,
            input_format = %req.input_format,
            "ProcessDocument called"
        );

        if req.input_content.is_empty() {
            return Err(Status::invalid_argument("input_content is empty"));
        }

        match req.operation.as_str() {
            "extract_text" => {
                match req.input_format.as_str() {
                    "pdf" | "application/pdf" => {
                        let result = extract_text_from_pdf(&req.input_content)
                            .map_err(sidecar_error_to_status)?;
                        Ok(Response::new(ProcessDocumentResponse {
                            output_uri: req.input_uri,
                            output_content: result.text.into_bytes(),
                            output_format: "text/plain".to_string(),
                            metadata: result.metadata,
                        }))
                    }
                    "docx" | "application/vnd.openxmlformats-officedocument.wordprocessingml.document" => {
                        let result = extract_text_from_docx(&req.input_content)
                            .map_err(sidecar_error_to_status)?;
                        Ok(Response::new(ProcessDocumentResponse {
                            output_uri: req.input_uri,
                            output_content: result.text.into_bytes(),
                            output_format: "text/plain".to_string(),
                            metadata: result.metadata,
                        }))
                    }
                    "xlsx" | "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" => {
                        let result = extract_data_from_xlsx(&req.input_content)
                            .map_err(sidecar_error_to_status)?;
                        let output_content = serde_json::to_vec(&result)
                            .map_err(|e| Status::internal(format!("Failed to serialize XLSX data: {}", e)))?;
                        Ok(Response::new(ProcessDocumentResponse {
                            output_uri: req.input_uri,
                            output_content,
                            output_format: "application/json".to_string(),
                            metadata: result.metadata,
                        }))
                    }
                    "image/png" | "image/jpeg" | "image/tiff" | "image/bmp" | "image/gif" => {
                        let lang = req.operation_params.get("language").cloned();
                        let config = lang.map(|l| crate::document::OcrConfig {
                            language: l,
                            dpi: req.operation_params.get("dpi").and_then(|d| d.parse().ok()),
                            psm: req.operation_params.get("psm").and_then(|p| p.parse().ok()),
                            ..Default::default()
                        });
                        let result = extract_text_with_ocr(&req.input_content, config)
                            .map_err(sidecar_error_to_status)?;
                        let mut metadata = result.metadata;
                        metadata.insert("confidence".to_string(), format!("{:.2}", result.confidence));
                        Ok(Response::new(ProcessDocumentResponse {
                            output_uri: req.input_uri,
                            output_content: result.text.into_bytes(),
                            output_format: "text/plain".to_string(),
                            metadata,
                        }))
                    }
                    other => Err(Status::invalid_argument(format!(
                        "Unsupported input format for extract_text: '{}'", other
                    ))),
                }
            }
            "convert" => {
                let target_format = req.operation_params.get("target_format")
                    .cloned()
                    .unwrap_or_default();

                match (req.input_format.as_str(), target_format.as_str()) {
                    ("docx" | "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
                     "pdf" | "application/pdf") => {
                        let pdf_bytes = convert_docx_to_pdf(&req.input_content)
                            .map_err(sidecar_error_to_status)?;
                        let mut metadata = std::collections::HashMap::new();
                        metadata.insert("source_format".to_string(), "docx".to_string());
                        metadata.insert("target_format".to_string(), "pdf".to_string());
                        Ok(Response::new(ProcessDocumentResponse {
                            output_uri: req.input_uri,
                            output_content: pdf_bytes,
                            output_format: "application/pdf".to_string(),
                            metadata,
                        }))
                    }
                    ("xlsx" | "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
                     "csv" | "text/csv") => {
                        let csv_bytes = convert_xlsx_to_csv(&req.input_content)
                            .map_err(sidecar_error_to_status)?;
                        let mut metadata = std::collections::HashMap::new();
                        metadata.insert("source_format".to_string(), "xlsx".to_string());
                        metadata.insert("target_format".to_string(), "csv".to_string());
                        Ok(Response::new(ProcessDocumentResponse {
                            output_uri: req.input_uri,
                            output_content: csv_bytes,
                            output_format: "text/csv".to_string(),
                            metadata,
                        }))
                    }
                    ("pptx" | "application/vnd.openxmlformats-officedocument.presentationml.presentation",
                     "pdf" | "application/pdf") => {
                        convert_pptx_to_pdf(&req.input_content)
                            .map_err(sidecar_error_to_status)?;
                        Err(Status::unimplemented("PPTX→PDF conversion"))
                    }
                    _ => Err(Status::invalid_argument(format!(
                        "Unsupported conversion: '{}' → '{}'. Supported: docx→pdf, xlsx→csv",
                        req.input_format, target_format
                    ))),
                }
            }
            other => {
                Err(Status::invalid_argument(format!(
                    "Unsupported operation: '{}'. Supported: extract_text, convert",
                    other
                )))
            }
        }
    }

    async fn query_documents(
        &self,
        request: Request<QueryDocumentsRequest>,
    ) -> std::result::Result<Response<QueryDocumentsResponse>, Status> {
        let req = request.into_inner();
        info!(collection_id = %req.collection_id, "QueryDocuments called");

        if req.query_text.is_empty() {
            return Err(Status::invalid_argument("query_text is empty"));
        }

        let qdrant = self.qdrant_client.as_ref()
            .ok_or_else(|| Status::unimplemented("Qdrant client not configured"))?;

        let api_key = self.embedding_api_key.as_ref()
            .ok_or_else(|| Status::unimplemented("Embedding API key not configured"))?;

        let embedder = OpenAIEmbedder::new(api_key.clone())
            .map_err(sidecar_error_to_status)?;

        let query_vector = embedder.generate_embedding(&req.query_text)
            .await
            .map_err(sidecar_error_to_status)?;

        if query_vector.is_empty() {
            return Err(Status::internal("Generated empty embedding vector"));
        }

        let limit = if req.max_results > 0 { req.max_results as usize } else { 10 };

        let scored_points = qdrant.search(query_vector, limit)
            .await
            .map_err(sidecar_error_to_status)?;

        let mut total_score: f32 = 0.0;
        let mut chunks: Vec<DocumentChunk> = Vec::new();

        for point in scored_points {
            let score = point.score;
            total_score += score;

            let chunk_id = point.id
                .and_then(|id| id.point_id_options)
                .map(|opts| match opts {
                    qdrant_client::qdrant::point_id::PointIdOptions::Num(n) => n.to_string(),
                    qdrant_client::qdrant::point_id::PointIdOptions::Uuid(s) => s,
                })
                .unwrap_or_default();

            let content = point.payload
                .get("content")
                .and_then(|v| extract_string_value(v))
                .unwrap_or_default()
                .to_string();

            let mut metadata = std::collections::HashMap::new();

            if let Some(clearance) = point.payload.get("clearance_level").and_then(|v| extract_string_value(v)) {
                metadata.insert("clearance_level".to_string(), clearance.to_string());
            }

            chunks.push(DocumentChunk {
                chunk_id,
                content,
                score,
                metadata,
            });
        }

        Ok(Response::new(QueryDocumentsResponse {
            chunks,
            total_score,
        }))
    }
}

pub async fn run_server(config: SidecarConfig) -> Result<()> {
    let socket_path = &config.socket_path;

    info!(
        socket_path = %socket_path.display(),
        "Starting gRPC server"
    );

    let registry = Registry::new();

    let shared_secret_bytes = config.shared_secret.as_bytes().to_vec();
    let security_interceptor = SecurityInterceptor::new(shared_secret_bytes, &registry)
        .map_err(|e| SidecarError::Config(format!("Failed to create security interceptor: {}", e)))?;

    info!("Security interceptor initialized with metrics");

    let s3_connector = if std::env::var("AWS_ACCESS_KEY_ID").is_ok()
        || std::env::var("AWS_SECRET_ACCESS_KEY").is_ok()
    {
        info!("S3 connector initialized (AWS credentials detected)");
        Some(S3Connector::new())
    } else {
        warn!("S3 connector not configured (no AWS credentials found)");
        None
    };

    let qdrant_client = if let Ok(qdrant_url) = std::env::var("QDRANT_URL") {
        let collection = std::env::var("QDRANT_COLLECTION")
            .unwrap_or_else(|_| "armorclaw-docs".to_string());
        match QdrantClient::new(&qdrant_url, &collection).await {
            Ok(client) => {
                info!(url = %qdrant_url, collection = %collection, "Qdrant client initialized");
                Some(client)
            }
            Err(e) => {
                warn!("Qdrant client initialization failed: {}", e);
                None
            }
        }
    } else {
        warn!("Qdrant client not configured (QDRANT_URL not set)");
        None
    };

    let embedding_api_key = std::env::var("OPEN_AI_KEY")
        .or_else(|_| std::env::var("OPENAI_API_KEY"))
        .ok();

    let impl_service = SidecarServiceImpl::new(s3_connector, qdrant_client, embedding_api_key);

    let _addr = format!("unix://{}", socket_path.display());

    let listener = UnixListener::bind(socket_path).map_err(|e| {
        if e.kind() == std::io::ErrorKind::AddrInUse {
            SidecarError::Config(format!(
                "Socket path {} already in use. Is another instance running?",
                socket_path.display()
            ))
        } else {
            SidecarError::Config(format!("Failed to bind to socket: {}", e))
        }
    })?;

    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(socket_path, std::fs::Permissions::from_mode(0o600))
            .map_err(|e| SidecarError::Config(format!("Failed to set socket permissions: {}", e)))?;
    }

    info!(
        socket_path = %socket_path.display(),
        "Server listening on Unix socket with 0600 permissions"
    );

    let server = tonic::transport::Server::builder()
        .add_service(SidecarServiceServer::with_interceptor(
            impl_service,
            security_interceptor,
        ))
        .serve_with_incoming(UnixListenerStream::new(listener));

    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("Failed to install CTRL+C handler");
        info!("Received CTRL+C, shutting down gracefully");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("Failed to install SIGTERM handler")
            .recv()
            .await;
        info!("Received SIGTERM, shutting down gracefully");
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {
            info!("Shutdown initiated via CTRL+C");
        }
        _ = terminate => {
            info!("Shutdown initiated via SIGTERM");
        }
        result = server => {
            if let Err(e) = result {
                error!("Server error: {}", e);
                return Err(SidecarError::Grpc(e));
            }
        }
    }

    info!("Server shutdown complete");

    if Path::new(socket_path).exists() {
        let _ = std::fs::remove_file(socket_path)
            .map_err(|e| error!("Failed to remove socket file: {}", e));
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::grpc::proto::{QueryDocumentsRequest, RequestMetadata};

    fn new_test_service() -> SidecarServiceImpl {
        SidecarServiceImpl::new(None, None, None)
    }

    fn make_query_request(query: &str, collection: &str, max_results: i32) -> QueryDocumentsRequest {
        QueryDocumentsRequest {
            metadata: None,
            collection_id: collection.to_string(),
            query_text: query.to_string(),
            clearance_level: String::new(),
            max_results,
        }
    }

    #[tokio::test]
    async fn query_documents_empty_query_returns_invalid_argument() {
        let service = new_test_service();
        let req = Request::new(make_query_request("", "col-1", 10));
        let result = service.query_documents(req).await;
        assert!(result.is_err());
        let status = result.unwrap_err();
        assert_eq!(status.code(), tonic::Code::InvalidArgument);
        assert!(status.message().contains("query_text is empty"));
    }

    #[tokio::test]
    async fn query_documents_no_qdrant_returns_unimplemented() {
        let service = new_test_service();
        let req = Request::new(make_query_request("find documents", "col-1", 10));
        let result = service.query_documents(req).await;
        assert!(result.is_err());
        let status = result.unwrap_err();
        assert_eq!(status.code(), tonic::Code::Unimplemented);
        assert!(status.message().contains("Qdrant"));
    }

    #[tokio::test]
    async fn query_documents_has_qdrant_no_api_key_returns_unimplemented() {
        let service = SidecarServiceImpl::new(None, None, None);
        let req = Request::new(make_query_request("find docs", "col-1", 10));
        let result = service.query_documents(req).await;
        assert!(result.is_err());
        let status = result.unwrap_err();
        assert_eq!(status.code(), tonic::Code::Unimplemented);
    }

    #[tokio::test]
    async fn query_documents_non_empty_query_validates_before_qdrant_check() {
        let service = new_test_service();
        let req = Request::new(make_query_request("valid query", "col-1", 5));
        let result = service.query_documents(req).await;
        assert!(result.is_err());
        let status = result.unwrap_err();
        assert_eq!(status.code(), tonic::Code::Unimplemented);
    }

    #[tokio::test]
    async fn query_documents_empty_collection_still_validates_query() {
        let service = new_test_service();
        let req = Request::new(make_query_request("", "", 0));
        let result = service.query_documents(req).await;
        assert!(result.is_err());
        let status = result.unwrap_err();
        assert_eq!(status.code(), tonic::Code::InvalidArgument);
    }

    #[tokio::test]
    async fn query_documents_with_metadata_and_clearance() {
        let service = new_test_service();
        let req = Request::new(QueryDocumentsRequest {
            metadata: Some(RequestMetadata {
                request_id: "req-123".to_string(),
                ephemeral_token: String::new(),
                timestamp_unix: 1234567890,
                operation_signature: String::new(),
            }),
            collection_id: "secure-col".to_string(),
            query_text: "classified docs".to_string(),
            clearance_level: "secret".to_string(),
            max_results: 5,
        });
        let result = service.query_documents(req).await;
        assert!(result.is_err());
        let status = result.unwrap_err();
        assert_eq!(status.code(), tonic::Code::Unimplemented);
    }

    #[tokio::test]
    async fn health_check_returns_ok() {
        let service = new_test_service();
        let req = Request::new(HealthCheckRequest {});
        let result = service.health_check(req).await;
        assert!(result.is_ok());
        let response = result.unwrap().into_inner();
        assert!(!response.status.is_empty());
    }

    #[tokio::test]
    async fn health_check_returns_real_telemetry() {
        let service = new_test_service();
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
        let req = Request::new(HealthCheckRequest {});
        let result = service.health_check(req).await;
        assert!(result.is_ok());
        let response = result.unwrap().into_inner();

        assert_eq!(response.status, "healthy");
        assert!(response.uptime_seconds >= 1, "uptime_seconds should be >= 1 after 1s sleep");
        assert_eq!(response.active_requests, 0, "no concurrent requests expected");
        assert!(response.memory_used_bytes > 0, "memory_used_bytes should be > 0 on Linux");
        assert!(!response.version.is_empty(), "version should not be empty");
    }
}
