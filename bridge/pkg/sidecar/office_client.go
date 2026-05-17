package sidecar

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// OfficeSocketPath is the canonical socket path for the Python office sidecar.
	// Scoped mount: host /run/armorclaw/sidecar-office → container /run/armorclaw,
	// so the socket appears at sidecar-office/sidecar-office.sock on the host.
	OfficeSocketPath = "/run/armorclaw/sidecar-office/sidecar-office.sock"
	OfficeMaxMsgSize = 100 * 1024 * 1024 // 100 MB
)

// NewOfficeClient creates a gRPC client pointing to the Python office sidecar socket.
func NewOfficeClient(piiConfig *PIIInterceptorConfig, tokenGen *TokenGenerator) *Client {
	config := &Config{
		SocketPath:     OfficeSocketPath,
		Timeout:        DefaultTimeout,
		MaxRetries:     DefaultMaxRetries,
		DialTimeout:    10 * time.Second,
		IdleTimeout:    5 * time.Minute,
		MaxMsgSize:     OfficeMaxMsgSize,
		PIIInterceptor: piiConfig,
		TokenGenerator: tokenGen,
	}
	return NewClient(config)
}

// RouteExtractText routes ExtractText requests based on 3-layer logic:
// Layer 0: Plain text formats → decode natively in Go (no sidecar)
// Layer 1: Compound magic+format switch → route to Python or Rust sidecar
// Layer 2: Strict drop → reject mismatched magic bytes + format claims
func RouteExtractText(
	ctx context.Context,
	req *ExtractTextRequest,
	officeClient *Client,
	rustClient *Client,
	javaClient *Client,
) (*ExtractTextResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "nil request")
	}

	// Layer 0: Bridge Bypass — plain text formats decoded natively in Go
	if isPlainText(req.DocumentFormat) {
		return &ExtractTextResponse{
			Text:      string(req.DocumentContent),
			PageCount: 1,
			Metadata:  map[string]string{"source": "bridge-native"},
		}, nil
	}

	content := req.DocumentContent
	if len(content) < 8 {
		return nil, status.Errorf(codes.InvalidArgument,
			"payload too small to contain valid magic bytes (need 8, got %d)", len(content))
	}

	// Layer 1: Magic byte detection (non-destructive 8-byte peek)
	isZIP := content[0] == 0x50 && content[1] == 0x4B && content[2] == 0x03 && content[3] == 0x04
	isOLE := content[0] == 0xD0 && content[1] == 0xCF && content[2] == 0x11 &&
		content[3] == 0xE0 && content[4] == 0xA1 && content[5] == 0xB1 &&
		content[6] == 0x1A && content[7] == 0xE1
	isPDF := content[0] == 0x25 && content[1] == 0x50 && content[2] == 0x44 && content[3] == 0x46

	// Format string classification
	f := strings.ToLower(req.DocumentFormat)
	isXlsx := strings.Contains(f, "spreadsheetml") || strings.HasSuffix(f, ".xlsx")
	isPptx := strings.Contains(f, "presentationml") || strings.HasSuffix(f, ".pptx")
	isDocx := strings.Contains(f, "wordprocessingml") || strings.HasSuffix(f, ".docx")
	isMsg := strings.Contains(f, "outlook") || strings.HasSuffix(f, ".msg")
	isDoc := (strings.Contains(f, "msword") && !strings.Contains(f, "wordprocessingml")) || strings.HasSuffix(f, ".doc")
	isXls := strings.Contains(f, "ms-excel") || strings.HasSuffix(f, ".xls")
	isPpt := strings.Contains(f, "ms-powerpoint") || strings.HasSuffix(f, ".ppt")
	isPdf := strings.Contains(f, "pdf") || strings.HasSuffix(f, ".pdf")

	// Rust sidecar routes (OpenXML: DOCX, XLSX, PPTX)
	// DOCX is Rust-only — no fallback.
	if isZIP && isDocx {
		return rustClient.ExtractText(ctx, req)
	}
	// XLSX and PPTX: try Rust first, fall back to Python sidecar if Rust socket
	// is unavailable (degraded mode — Rust sidecar deferred to v1.3).
	if isZIP && isXlsx {
		resp, err := rustClient.ExtractText(ctx, req)
		if err == nil {
			return resp, nil
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			fmt.Printf("WARN Rust sidecar unavailable for %s, falling back to Python sidecar (degraded)\n", req.DocumentFormat)
			return officeClient.ExtractText(ctx, req)
		}
		return nil, err
	}
	if isZIP && isPptx {
		resp, err := rustClient.ExtractText(ctx, req)
		if err == nil {
			return resp, nil
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			fmt.Printf("WARN Rust sidecar unavailable for %s, falling back to Python sidecar (degraded)\n", req.DocumentFormat)
			return officeClient.ExtractText(ctx, req)
		}
		return nil, err
	}

	// Python sidecar routes (legacy OLE formats only)
	if isOLE && isMsg {
		return officeClient.ExtractText(ctx, req)
	}
	if isOLE && isDoc {
		if javaClient == nil {
			return nil, status.Errorf(codes.Unavailable,
				"DOC/PPT extraction requires the Java sidecar but it is currently unavailable. Deploy sidecar-java for legacy Office format support or contact your administrator.")
		}
		return javaClient.ExtractText(ctx, req)
	}
	if isOLE && isXls {
		return officeClient.ExtractText(ctx, req)
	}
	if isOLE && isPpt {
		if javaClient == nil {
			return nil, status.Errorf(codes.Unavailable,
				"DOC/PPT extraction requires the Java sidecar but it is currently unavailable. Deploy sidecar-java for legacy Office format support or contact your administrator.")
		}
		return javaClient.ExtractText(ctx, req)
	}

	if isPDF && isPdf {
		return rustClient.ExtractText(ctx, req)
	}

	// Layer 2: Strict Drop Policy — reject mismatches
	if isZIP && (isMsg || isDoc || isXls || isPpt || isPdf) {
		return nil, status.Errorf(codes.InvalidArgument,
			"magic byte/format mismatch: ZIP container but format claims %q", req.DocumentFormat)
	}
	if isOLE && (isXlsx || isPptx || isDocx || isPdf) {
		return nil, status.Errorf(codes.InvalidArgument,
			"magic byte/format mismatch: OLE container but format claims %q", req.DocumentFormat)
	}
	if isPDF && !isPdf {
		return nil, status.Errorf(codes.InvalidArgument,
			"magic byte/format mismatch: PDF header but format claims %q", req.DocumentFormat)
	}

	return nil, status.Errorf(codes.InvalidArgument,
		"unrecognized document format: %q", req.DocumentFormat)
}

// isPlainText returns true for formats that can be decoded natively in Go.
func isPlainText(documentFormat string) bool {
	f := strings.ToLower(documentFormat)
	plainFormats := map[string]bool{
		"text/plain": true, "text/csv": true,
		"application/json": true, "text/markdown": true,
	}
	if plainFormats[f] {
		return true
	}
	return strings.HasSuffix(f, ".txt") || strings.HasSuffix(f, ".csv") ||
		strings.HasSuffix(f, ".json") || strings.HasSuffix(f, ".md")
}

// CheckRustSidecarHealth logs a warning if the Rust sidecar socket is absent.
func CheckRustSidecarHealth() {
	if _, err := os.Stat(DefaultSocketPath); os.IsNotExist(err) {
		slog.Warn("Rust sidecar socket not found — .docx and .pdf routing will fail until sidecar is deployed",
			"socket", DefaultSocketPath)
	}
}

// extractionMode tracks the current document extraction backend mode.
// Initialized to ExtractionDetecting and transitions after sidecar health probes.
var extractionMode ExtractionMode = ExtractionDetecting

// GetExtractionMode returns the current extraction mode for health reporting.
func GetExtractionMode() ExtractionMode {
	return extractionMode
}

// ProbeExtractionMode checks sidecar socket availability and transitions the
// extraction mode from "detecting" to the actual backend state.  It probes the
// Java sidecar first (preferred), then the Python sidecar (fallback).
// Call this once after startup — it is non-blocking and fast (os.Stat only).
func ProbeExtractionMode() {
	javaReachable := socketExists(JavaSocketPath)
	pythonReachable := socketExists(OfficeSocketPath)

	switch {
	case javaReachable:
		extractionMode = ExtractionJavaPrimary
		slog.Info("DOC/PPT extraction: java_primary", "socket", JavaSocketPath)
	case pythonReachable:
		extractionMode = ExtractionPythonFallback
		slog.Info("DOC/PPT extraction: python_fallback_degraded", "socket", OfficeSocketPath)
	default:
		extractionMode = ExtractionUnavailable
		slog.Warn("DOC/PPT extraction: unavailable — no DOC/PPT sidecar socket found",
			"java_socket", JavaSocketPath,
			"python_socket", OfficeSocketPath)
	}
}

// LogExtractionModeStartup emits the initial startup log line for extraction mode.
func LogExtractionModeStartup() {
	slog.Info("DOC/PPT extraction: detecting (will probe sidecars)")
}

// socketExists returns true if the given path exists as a file/socket.
func socketExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ProvisionOfficeSocketDir creates the socket directory for the Python office sidecar
// with permissions allowing UID 10001 to write.
func ProvisionOfficeSocketDir() error {
	dir := "/run/armorclaw/sidecar-office"
	if err := os.MkdirAll(dir, 0770); err != nil {
		return fmt.Errorf("failed to create office socket dir: %w", err)
	}
	if err := os.Chown(dir, 10001, 10001); err != nil {
		slog.Warn("failed to chown office socket dir (non-root?)", "error", err)
	}
	return nil
}
