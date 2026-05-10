package sidecar

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeRoutingReq(format string, magic []byte) *ExtractTextRequest {
	content := make([]byte, 16)
	copy(content, magic)
	return &ExtractTextRequest{
		DocumentFormat:  format,
		DocumentContent: content,
	}
}

func TestRouteExtractText_NativeTextTXT(t *testing.T) {
	req := &ExtractTextRequest{
		DocumentFormat:  "text/plain",
		DocumentContent: []byte("hello world"),
	}
	resp, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Text)
	}
	if resp.Metadata["source"] != "bridge-native" {
		t.Error("expected bridge-native source")
	}
}

func TestRouteExtractText_NativeTextCSV(t *testing.T) {
	req := &ExtractTextRequest{
		DocumentFormat:  "text/csv",
		DocumentContent: []byte("a,b,c"),
	}
	resp, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "a,b,c" {
		t.Errorf("expected 'a,b,c', got %q", resp.Text)
	}
}

func TestRouteExtractText_NativeTextJSON(t *testing.T) {
	req := &ExtractTextRequest{
		DocumentFormat:  "application/json",
		DocumentContent: []byte(`{"key":"val"}`),
	}
	resp, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != `{"key":"val"}` {
		t.Errorf("unexpected text: %q", resp.Text)
	}
}

func TestRouteExtractText_NativeTextMD(t *testing.T) {
	req := &ExtractTextRequest{
		DocumentFormat:  "text/markdown",
		DocumentContent: []byte("# Hello"),
	}
	resp, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "# Hello" {
		t.Errorf("unexpected text: %q", resp.Text)
	}
}

func setupRoutingClients(t *testing.T) (*Client, *mockServer, *Client, *mockServer) {
	t.Helper()

	officeSrv, officeMock, officeSock := setupTestServer(t)
	t.Cleanup(func() { officeSrv.Stop() })

	rustSrv, rustMock, rustSock := setupTestServer(t)
	t.Cleanup(func() { rustSrv.Stop() })

	officeClient := NewClient(&Config{
		SocketPath:  officeSock,
		Timeout:     5 * time.Second,
		MaxRetries:  1,
		DialTimeout: 5 * time.Second,
	})

	rustClient := NewClient(&Config{
		SocketPath:  rustSock,
		Timeout:     5 * time.Second,
		MaxRetries:  1,
		DialTimeout: 5 * time.Second,
	})

	return officeClient, officeMock, rustClient, rustMock
}

func setupRoutingClientsWithJava(t *testing.T) (*Client, *mockServer, *Client, *mockServer, *Client, *mockServer) {
	t.Helper()

	officeSrv, officeMock, officeSock := setupTestServer(t)
	t.Cleanup(func() { officeSrv.Stop() })

	rustSrv, rustMock, rustSock := setupTestServer(t)
	t.Cleanup(func() { rustSrv.Stop() })

	javaSrv, javaMock, javaSock := setupTestServer(t)
	t.Cleanup(func() { javaSrv.Stop() })

	officeClient := NewClient(&Config{
		SocketPath:  officeSock,
		Timeout:     5 * time.Second,
		MaxRetries:  1,
		DialTimeout: 5 * time.Second,
	})

	rustClient := NewClient(&Config{
		SocketPath:  rustSock,
		Timeout:     5 * time.Second,
		MaxRetries:  1,
		DialTimeout: 5 * time.Second,
	})

	javaClient := NewClient(&Config{
		SocketPath:  javaSock,
		Timeout:     5 * time.Second,
		MaxRetries:  1,
		DialTimeout: 5 * time.Second,
	})

	return officeClient, officeMock, rustClient, rustMock, javaClient, javaMock
}

func TestRouteExtractText_XLSX_RoutesToRust(t *testing.T) {
	office, officeMock, rust, _ := setupRoutingClients(t)
	zipMagic := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	req := makeRoutingReq("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", zipMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if officeMock.extractCalled {
		t.Error("office client should NOT have been called for .xlsx")
	}
}

func TestRouteExtractText_PPTX_RoutesToRust(t *testing.T) {
	office, officeMock, rust, _ := setupRoutingClients(t)
	zipMagic := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	req := makeRoutingReq("application/vnd.openxmlformats-officedocument.presentationml.presentation", zipMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if officeMock.extractCalled {
		t.Error("office client should NOT have been called for .pptx")
	}
}

func TestRouteExtractText_MSG_RoutesToPython(t *testing.T) {
	office, _, rust, rustMock := setupRoutingClients(t)
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/vnd.ms-outlook", oleMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rustMock.extractCalled {
		t.Error("rust client should NOT have been called for .msg")
	}
}

func TestRouteExtractText_DOC_RequiresJava_NoSidecar(t *testing.T) {
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/msword", oleMagic)

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for .doc without javaClient")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}
}

func TestRouteExtractText_XLS_RoutesToPython(t *testing.T) {
	office, _, rust, rustMock := setupRoutingClients(t)
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/vnd.ms-excel", oleMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rustMock.extractCalled {
		t.Error("rust client should NOT have been called for .xls")
	}
}

func TestRouteExtractText_PPT_RequiresJava_NoSidecar(t *testing.T) {
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/vnd.ms-powerpoint", oleMagic)

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for .ppt without javaClient")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}
}

func TestRouteExtractText_DOCX_RoutesToRust(t *testing.T) {
	office, officeMock, rust, _ := setupRoutingClients(t)
	zipMagic := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	req := makeRoutingReq("application/vnd.openxmlformats-officedocument.wordprocessingml.document", zipMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if officeMock.extractCalled {
		t.Error("office client should NOT have been called for .docx")
	}
}

func TestRouteExtractText_PDF_RoutesToRust(t *testing.T) {
	office, officeMock, rust, _ := setupRoutingClients(t)
	pdfMagic := []byte{0x25, 0x50, 0x44, 0x46, 0x00, 0x00, 0x00, 0x00}
	req := makeRoutingReq("application/pdf", pdfMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if officeMock.extractCalled {
		t.Error("office client should NOT have been called for .pdf")
	}
}

func TestRouteExtractText_MismatchZIPMsg_StrictDrop(t *testing.T) {
	zipMagic := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}
	req := makeRoutingReq("application/vnd.ms-outlook", zipMagic)

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for ZIP+msg mismatch")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestRouteExtractText_MismatchOLEXLSX_StrictDrop(t *testing.T) {
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", oleMagic)

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for OLE+xlsx mismatch")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestRouteExtractText_MismatchPDFMsg_StrictDrop(t *testing.T) {
	pdfMagic := []byte{0x25, 0x50, 0x44, 0x46, 0x00, 0x00, 0x00, 0x00}
	req := makeRoutingReq("application/vnd.ms-outlook", pdfMagic)

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for PDF+msg mismatch")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestRouteExtractText_UnknownFormat_StrictDrop(t *testing.T) {
	req := makeRoutingReq("application/unknown", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestRouteExtractText_ShortBuffer(t *testing.T) {
	req := &ExtractTextRequest{
		DocumentFormat:  "application/vnd.ms-outlook",
		DocumentContent: []byte{0x50},
	}

	_, err := RouteExtractText(context.Background(), req, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for short buffer")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestRouteExtractText_DOC_RoutesToJava(t *testing.T) {
	office, officeMock, _, _, java, javaMock := setupRoutingClientsWithJava(t)
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/msword", oleMagic)

	_, err := RouteExtractText(context.Background(), req, office, nil, java)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !javaMock.extractCalled {
		t.Error("java client should have been called for .doc")
	}
	if officeMock.extractCalled {
		t.Error("office client should NOT have been called for .doc when javaClient is present")
	}
}

func TestRouteExtractText_PPT_RoutesToJava(t *testing.T) {
	office, officeMock, _, _, java, javaMock := setupRoutingClientsWithJava(t)
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/vnd.ms-powerpoint", oleMagic)

	_, err := RouteExtractText(context.Background(), req, office, nil, java)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !javaMock.extractCalled {
		t.Error("java client should have been called for .ppt")
	}
	if officeMock.extractCalled {
		t.Error("office client should NOT have been called for .ppt when javaClient is present")
	}
}

func TestRouteExtractText_DOC_RequiresJava(t *testing.T) {
	office, _, rust, _ := setupRoutingClients(t)
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/msword", oleMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err == nil {
		t.Fatal("expected error when javaClient is nil for .doc")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}
	expected := "DOC/PPT extraction requires the Java sidecar but it is currently unavailable"
	if !strings.Contains(st.Message(), expected) {
		t.Errorf("error message should contain %q, got %q", expected, st.Message())
	}
}

func TestRouteExtractText_PPT_RequiresJava(t *testing.T) {
	office, _, rust, _ := setupRoutingClients(t)
	oleMagic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	req := makeRoutingReq("application/vnd.ms-powerpoint", oleMagic)

	_, err := RouteExtractText(context.Background(), req, office, rust, nil)
	if err == nil {
		t.Fatal("expected error when javaClient is nil for .ppt")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}
	expected := "DOC/PPT extraction requires the Java sidecar but it is currently unavailable"
	if !strings.Contains(st.Message(), expected) {
		t.Errorf("error message should contain %q, got %q", expected, st.Message())
	}
}

func TestIsPlainText(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"text/plain", true},
		{"text/csv", true},
		{"application/json", true},
		{"text/markdown", true},
		{"text/PLAIN", true},
		{"file.txt", true},
		{"file.csv", true},
		{"file.json", true},
		{"file.md", true},
		{"application/pdf", false},
		{"application/vnd.ms-outlook", false},
	}
	for _, tt := range tests {
		got := isPlainText(tt.format)
		if got != tt.want {
			t.Errorf("isPlainText(%q) = %v, want %v", tt.format, got, tt.want)
		}
	}
}
