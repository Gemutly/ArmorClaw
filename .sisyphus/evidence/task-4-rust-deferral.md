# Rust Sidecar Deferral — BEATO v1.1

## Decision
**DEFERRED** — Rust sidecar will not be deployed in this sprint.

## What Was Deferred
- Dockerfile for the Rust sidecar (`sidecar/Dockerfile`)
- Docker Compose entry (`deploy/docker-compose.sidecar-rust.yml`)
- Hardened container deployment with: network_mode: none, cap_drop: ALL, read_only: true, no-new-privileges, Unix socket only, resource limits, YARA-before-sidecar enforcement

## Why
- No Dockerfile exists for the Rust sidecar anywhere in the repository
- No docker-compose entry exists for containerized deployment
- Deployment is a separate effort requiring: Dockerfile creation, multi-stage build (builder + runtime), hardened compose config, gRPC socket provisioning, integration testing
- Current sprint focuses on auth enforcement and runtime verification, not new infrastructure

## What Works (Library State)
- `sidecar/Cargo.toml`: Complete Rust crate with all dependencies (tonic, tokio, aws-sdk-s3, etc.)
- 252 library tests passing, 8 ignored, 0 failing (per README)
- Full document processing: PDF, DOCX, XLSX, OCR
- Full cloud connectors: S3, SharePoint (Azure disabled)
- Security: token validation, HMAC, rate limiting, circuit breakers
- gRPC server implementation ready for Unix socket deployment

## What's Missing
- Containerized runtime (no Dockerfile)
- gRPC-over-Unix-socket deployment in Docker
- Hardened Docker security config
- Integration with Go Bridge sidecar routing
- CI/CD pipeline for Rust sidecar image builds

## Office Score Ceiling
- **Maximum Office score without Rust**: 15/25
  - Python sidecar handles: XLSX, PPTX, DOC, PPT, MSG
  - Go native handles: plain text (Layer 0)
  - Missing: PDF advanced operations (split, merge, streaming), S3 upload/download, DOCX advanced editing
- **Maximum Office score with Rust**: 20-25/25
  - Adds: PDF split/merge, S3 streaming, DOCX editing, circuit breaker resilience

## Total BEATO Impact
- **Ceiling without Rust**: ~90/100
- **Potential with Rust**: ~95+/100
- The 5-point gap is entirely in the Office pillar (advanced document ops and S3 streaming)

## Compensating Controls
- Python sidecar provides document extraction for common formats
- Go Bridge handles plain text natively (Layer 0 bypass)
- YARA content disarm is applied regardless of sidecar choice

## Follow-Up Task
- Create `sidecar/Dockerfile` with multi-stage build (Rust builder + minimal runtime)
- Create `deploy/docker-compose.sidecar-rust.yml` with hardened config (follow `docker-compose.sidecar-py.yml` pattern)
- Wire into Go Bridge sidecar routing alongside Python sidecar
- Add to CI/CD pipeline (sidecar.yml workflow)
- Target: next hardening sprint
