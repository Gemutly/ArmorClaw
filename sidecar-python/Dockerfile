# ---- Stage 1: Builder — proto compilation + stub generation ----
FROM python:3.12-slim AS builder
WORKDIR /build
RUN pip install --no-cache-dir grpcio-tools
COPY proto/sidecar.proto ./proto/
RUN python -m grpc_tools.protoc \
    -I. \
    --python_out=. \
    --grpc_python_out=. \
    proto/sidecar.proto
COPY proto/__init__.py ./proto/

# ---- Stage 2: Runtime — minimal production image ----
FROM python:3.12-slim
WORKDIR /app
RUN pip install --no-cache-dir grpcio grpcio-health-checking 'markitdown[outlook,xlsx,pptx]' olefile xlrd
COPY --from=builder /build/proto/ ./proto/
COPY worker.py interceptor.py ./
RUN mkdir -p /run/armorclaw /tmp/office_worker
CMD ["python", "worker.py"]
