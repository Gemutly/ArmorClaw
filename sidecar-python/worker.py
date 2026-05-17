"""ArmorClaw Python Office Sidecar — MarkItDown gRPC server.

Handles .xlsx, .pptx (OpenXML) and .msg, .doc, .xls, .ppt (OLE) document
conversion to Markdown via Microsoft MarkItDown. Implements the existing
SidecarService.ExtractText unary RPC contract from sidecar.proto.
"""

import io
import os
import sys
import tempfile
import threading
import time
import traceback
from concurrent import futures

import grpc
from markitdown import MarkItDown, StreamInfo

from proto import sidecar_pb2, sidecar_pb2_grpc
from interceptor import TokenInterceptor, load_shared_secret

# Threshold for in-memory vs temp-file conversion (10 MB)
_THRESHOLD_BYTES = 10 * 1024 * 1024

# Crash-only TTL: exit after this many requests to recycle the container
MAX_REQUESTS = 50

# gRPC message size limits (100 MB — must be <= Go client's 256 MB)
_MAX_MESSAGE_BYTES = 100 * 1024 * 1024

SERVER_VERSION = "1.0.0"

# Format MIME → extension mapping (Python sidecar scope only)
FORMAT_MAP = {
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
    "application/vnd.ms-outlook": ".msg",
    "application/msword": ".doc",
    "application/vnd.ms-excel": ".xls",
    "application/vnd.ms-powerpoint": ".ppt",
}

# Extension-based fallback matching (lowercased)
EXTENSION_MAP = {
    ".xlsx": ".xlsx",
    ".pptx": ".pptx",
    ".msg": ".msg",
    ".doc": ".doc",
    ".xls": ".xls",
    ".ppt": ".ppt",
}

_start_time = time.time()
_request_count = 0
_request_lock = threading.Lock()
_drain_event = threading.Event()


def _increment_and_check():
    global _request_count
    with _request_lock:
        _request_count += 1
        if _request_count >= MAX_REQUESTS:
            _drain_event.set()


class OfficeSidecarServicer(sidecar_pb2_grpc.SidecarServiceServicer):

    def HealthCheck(self, request, context):
        _increment_and_check()
        uptime = int(time.time() - _start_time)
        with _request_lock:
            active = _request_count
        return sidecar_pb2.HealthCheckResponse(
            status="SERVING",
            uptime_seconds=uptime,
            active_requests=active,
            memory_used_bytes=0,
            version=SERVER_VERSION,
        )

    def ExtractText(self, request, context):
        _increment_and_check()
        document_content = request.document_content
        document_format = request.document_format

        if not document_content:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "empty document_content")

        extension = self._resolve_extension(document_format)
        if extension is None:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                f"unsupported document_format: {document_format!r}",
            )

        try:
            md = MarkItDown()

            if len(document_content) < _THRESHOLD_BYTES:
                result = self._convert_in_memory(md, document_content, extension, document_format)
            else:
                result = self._convert_via_tmpfile(md, document_content, extension)

            return sidecar_pb2.ExtractTextResponse(
                text=result.markdown if result.markdown else "",
                page_count=0,
                metadata={},
            )

        except grpc.RpcError:
            raise
        except Exception as e:
            exc_name = type(e).__name__
            if "UnsupportedFormat" in exc_name or "MissingDependency" in exc_name:
                context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
            context.abort(grpc.StatusCode.INTERNAL, f"conversion failed: {exc_name}: {e}")

    def _resolve_extension(self, document_format: str):
        fmt_lower = document_format.lower().strip()
        if fmt_lower in FORMAT_MAP:
            return FORMAT_MAP[fmt_lower]
        for ext, mapped in EXTENSION_MAP.items():
            if fmt_lower.endswith(ext):
                return mapped
        return None

    def _convert_in_memory(self, md, content: bytes, extension: str, mimetype: str):
        stream = io.BytesIO(content)
        stream_info = StreamInfo(extension=extension, mimetype=mimetype)
        return md.convert_stream(stream, stream_info=stream_info)

    def _convert_via_tmpfile(self, md, content: bytes, extension: str):
        tmpdir = "/tmp/office_worker"
        os.makedirs(tmpdir, exist_ok=True)
        tmp = tempfile.NamedTemporaryFile(dir=tmpdir, delete=False, suffix=extension)
        try:
            tmp.write(content)
            tmp.flush()
            tmp.close()
            return md.convert(tmp.name)
        finally:
            try:
                os.remove(tmp.name)
            except OSError:
                pass

    # Storage RPCs — not implemented for Python sidecar, return UNIMPLEMENTED
    def UploadBlob(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "UploadBlob not supported by office sidecar")

    def DownloadBlob(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "DownloadBlob not supported by office sidecar")

    def ListBlobs(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "ListBlobs not supported by office sidecar")

    def DeleteBlob(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "DeleteBlob not supported by office sidecar")

    def ProcessDocument(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "ProcessDocument not supported by office sidecar")


def serve():
    shared_secret = load_shared_secret()
    interceptor = TokenInterceptor(shared_secret)

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=4),
        interceptors=[interceptor],
        options=[
            ("grpc.max_receive_message_length", _MAX_MESSAGE_BYTES),
            ("grpc.max_send_message_length", _MAX_MESSAGE_BYTES),
        ],
    )

    sidecar_pb2_grpc.add_SidecarServiceServicer_to_server(
        OfficeSidecarServicer(), server
    )

    os.umask(0o077)
    socket_path = os.environ.get("SIDECAR_SOCKET", "/run/armorclaw/sidecar-office.sock")
    server.add_insecure_port(f"unix://{socket_path}")
    os.chmod(socket_path, 0o600)

    server.start()
    print(f"Office sidecar listening on unix://{socket_path} (version {SERVER_VERSION})")

    _drain_event.wait()
    print(f"TTL reached ({MAX_REQUESTS} requests) — draining with 30s grace...")
    stop_event = server.stop(grace=30)
    stop_event.wait()
    print("Server stopped.")


if __name__ == "__main__":
    serve()
