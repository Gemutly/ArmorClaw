package email

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/armorclaw/bridge/pkg/eventbus"
	"github.com/armorclaw/bridge/pkg/logger"
)

type mockEmailStorage struct {
	mu                sync.Mutex
	emails            map[string][]byte
	attachments       map[string]map[string][]byte
	extractedTexts    map[string]map[string]string
	storeTextErr      error
	storeTextCalled   bool
	storeTextFilename string
	storeTextContent  string
}

func newMockEmailStorage() *mockEmailStorage {
	return &mockEmailStorage{
		emails:          make(map[string][]byte),
		attachments:     make(map[string]map[string][]byte),
		extractedTexts:  make(map[string]map[string]string),
	}
}

func (m *mockEmailStorage) StoreEmail(emailID string, rawEmail []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emails[emailID] = rawEmail
	return nil
}

func (m *mockEmailStorage) StoreAttachment(emailID, filename string, content []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.attachments[emailID] == nil {
		m.attachments[emailID] = make(map[string][]byte)
	}
	m.attachments[emailID][filename] = content
	return emailID + "-" + filename, nil
}

func (m *mockEmailStorage) GetAttachment(fileID string) ([]byte, error) {
	return nil, nil
}

func (m *mockEmailStorage) DeleteEmail(emailID string) error {
	return nil
}

func (m *mockEmailStorage) StoreAttachmentText(emailID, filename, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeTextCalled = true
	m.storeTextFilename = filename
	m.storeTextContent = text
	if m.storeTextErr != nil {
		return m.storeTextErr
	}
	if m.extractedTexts[emailID] == nil {
		m.extractedTexts[emailID] = make(map[string]string)
	}
	m.extractedTexts[emailID][filename] = text
	return nil
}

func (m *mockEmailStorage) getExtractedText(emailID, filename string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.extractedTexts[emailID] == nil {
		return "", false
	}
	t, ok := m.extractedTexts[emailID][filename]
	return t, ok
}

func TestMIMEToFormatMapping(t *testing.T) {
	expected := map[string]string{
		"application/pdf":                                                          "pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":  "docx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":        "xlsx",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
		"application/vnd.ms-excel":                                                 "xls",
		"application/vnd.ms-powerpoint":                                            "ppt",
		"application/msword":                                                       "doc",
		"application/vnd.ms-outlook":                                               "msg",
	}

	for mime, expectedFormat := range expected {
		format, ok := mimeToFormat[mime]
		if !ok {
			t.Errorf("mimeToFormat missing entry for %q", mime)
			continue
		}
		if format != expectedFormat {
			t.Errorf("mimeToFormat[%q] = %q, want %q", mime, format, expectedFormat)
		}
	}

	unexpectedMimes := []string{
		"image/png",
		"image/jpeg",
		"video/mp4",
		"audio/mpeg",
		"text/plain",
		"application/octet-stream",
	}
	for _, mime := range unexpectedMimes {
		if _, ok := mimeToFormat[mime]; ok {
			t.Errorf("mimeToFormat should not contain %q", mime)
		}
	}
}

func TestAttachmentExtraction_PDF(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) { return true, nil }

	attachments := []ParsedAttachment{
		{
			Filename:    "report.pdf",
			Content:     []byte("%PDF-1.4 test pdf content"),
			ContentType: "application/pdf",
			Size:        24,
		},
	}

	s.extractAttachments("email-pdf-test", attachments)

	text, ok := storage.getExtractedText("email-pdf-test", "report.pdf")
	if !ok {
		t.Fatal("expected extracted text to be stored for PDF attachment")
	}
	if text != "%PDF-1.4 test pdf content" {
		t.Errorf("extracted text = %q, want original content (native text bypass)", text)
	}
}

func TestAttachmentExtraction_Image(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) { return true, nil }

	attachments := []ParsedAttachment{
		{
			Filename:    "photo.png",
			Content:     []byte("fake png data"),
			ContentType: "image/png",
			Size:        14,
		},
	}

	s.extractAttachments("email-image-test", attachments)

	_, ok := storage.getExtractedText("email-image-test", "photo.png")
	if ok {
		t.Error("expected no extraction for image attachment")
	}
}

func TestAttachmentExtraction_Oversized(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) { return true, nil }

	attachments := []ParsedAttachment{
		{
			Filename:    "big.pdf",
			Content:     make([]byte, 10*1024*1024+1),
			ContentType: "application/pdf",
			Size:        10*1024*1024 + 1,
		},
	}

	s.extractAttachments("email-oversized-test", attachments)

	_, ok := storage.getExtractedText("email-oversized-test", "big.pdf")
	if ok {
		t.Error("expected no extraction for oversized attachment")
	}
}

func TestAttachmentExtraction_SidecarDown(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) { return true, nil }

	attachments := []ParsedAttachment{
		{
			Filename:    "report.docx",
			Content:     []byte("PK\x03\x04 fake docx content"),
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			Size:        26,
		},
	}

	s.extractAttachments("email-sidecar-down-test", attachments)

	_, ok := storage.getExtractedText("email-sidecar-down-test", "report.docx")
	if ok {
		t.Error("expected no extraction when sidecar is unavailable for non-native format")
	}
}

func TestAttachmentExtraction_AsyncInIngestEmail(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) { return true, nil }

	rawEmail := []byte("From: sender@test.com\r\nTo: recv@test.com\r\nSubject: Test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=BOUNDARY\r\n\r\n--BOUNDARY\r\nContent-Type: text/plain\r\n\r\nHello\r\n--BOUNDARY\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"doc.pdf\"\r\n\r\n%PDF-1.4 test content\r\n--BOUNDARY--\r\n")

	resp := s.IngestEmail(nil, rawEmail, "sender@test.com", "recv@test.com", "queue-001")

	if !resp.Accepted {
		t.Fatalf("expected email to be accepted, got rejection: %q", resp.RejectionReason)
	}

	time.Sleep(200 * time.Millisecond)

	text, ok := storage.getExtractedText(resp.EmailID, "doc.pdf")
	if !ok {
		t.Fatal("expected extracted text to be stored asynchronously")
	}
	if text != "%PDF-1.4 test content" {
		t.Errorf("extracted text = %q, want %q", text, "%PDF-1.4 test content")
	}
}

func TestLocalFS_StoreAttachmentText(t *testing.T) {
	baseDir := t.TempDir()
	s := NewLocalFSEmailStorage(baseDir)

	err := s.StoreAttachmentText("email-001", "report.pdf", "extracted text content")
	if err != nil {
		t.Fatalf("StoreAttachmentText: %v", err)
	}

	textPath := filepath.Join(baseDir, "extracted-text", "email-001", "report.pdf.txt")
	data, err := os.ReadFile(textPath)
	if err != nil {
		t.Fatalf("read stored text: %v", err)
	}
	if string(data) != "extracted text content" {
		t.Errorf("stored text = %q, want %q", string(data), "extracted text content")
	}
}

func buildMIMEWithAttachment(filename, contentType, body string) []byte {
	return []byte("From: sender@test.com\r\nTo: recv@test.com\r\nSubject: YARA Test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=YBOUND\r\n\r\n--YBOUND\r\nContent-Type: text/plain\r\n\r\nHello\r\n--YBOUND\r\nContent-Type: " + contentType + "\r\nContent-Disposition: attachment; filename=\"" + filename + "\"\r\n\r\n" + body + "\r\n--YBOUND--\r\n")
}

func TestIngest_YARANotInitialized(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) {
		return false, fmt.Errorf("YARA not initialized")
	}

	rawEmail := buildMIMEWithAttachment("invoice.pdf", "application/pdf", "%PDF-1.4 malware-bytes")
	resp := s.IngestEmail(nil, rawEmail, "sender@test.com", "recv@test.com", "queue-yara-err")

	if !resp.Accepted {
		t.Fatalf("expected email to be accepted when YARA returns error, got rejection: %q", resp.RejectionReason)
	}
}

func TestIngest_YARABlock(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) {
		return false, nil
	}

	rawEmail := buildMIMEWithAttachment("evil.exe", "application/octet-stream", "MZ\x90\x00 malicious")
	resp := s.IngestEmail(nil, rawEmail, "bad@test.com", "recv@test.com", "queue-yara-block")

	if resp.Accepted {
		t.Fatal("expected email to be rejected when YARA detects malware")
	}
	if resp.RejectionReason != "malware detected" {
		t.Errorf("rejection reason = %q, want %q", resp.RejectionReason, "malware detected")
	}
}

func TestIngest_YARAClean(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) {
		return true, nil
	}

	rawEmail := buildMIMEWithAttachment("clean.pdf", "application/pdf", "%PDF-1.4 clean content")
	resp := s.IngestEmail(nil, rawEmail, "sender@test.com", "recv@test.com", "queue-yara-clean")

	if !resp.Accepted {
		t.Fatalf("expected email to be accepted when YARA reports clean, got rejection: %q", resp.RejectionReason)
	}
}

func TestIngest_TempFileCleanup(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	var scannedPaths []string
	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) {
		scannedPaths = append(scannedPaths, filePath)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("temp file %q should exist during scan but does not", filePath)
		}
		return true, nil
	}

	rawEmail := buildMIMEWithAttachment("doc.pdf", "application/pdf", "%PDF-1.4 cleanup-test")
	resp := s.IngestEmail(nil, rawEmail, "sender@test.com", "recv@test.com", "queue-cleanup")

	if !resp.Accepted {
		t.Fatalf("expected email accepted, got: %q", resp.RejectionReason)
	}

	if len(scannedPaths) == 0 {
		t.Fatal("expected YARA scan to be called")
	}

	for _, p := range scannedPaths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("temp file %q should have been removed after scan", p)
			os.Remove(p)
		}
	}
}

func TestAttachmentExtraction_MultipleAttachments(t *testing.T) {
	storage := newMockEmailStorage()
	bus := eventbus.NewEventBus(eventbus.DefaultConfig())
	log, _ := logger.New(logger.Config{Output: "stdout"})

	s := NewIngestServer(IngestServerConfig{
		Storage: storage,
		Bus:     bus,
		Log:     log,
	})
	s.yaraScan = func(filePath string) (bool, error) { return true, nil }

	attachments := []ParsedAttachment{
		{
			Filename:    "doc.pdf",
			Content:     []byte("%PDF-1.4 pdf data"),
			ContentType: "application/pdf",
			Size:        16,
		},
		{
			Filename:    "photo.jpg",
			Content:     []byte("fake jpeg"),
			ContentType: "image/jpeg",
			Size:        10,
		},
		{
			Filename:    "data.xlsx",
			Content:     []byte("PK\x03\x04 xlsx data"),
			ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			Size:        16,
		},
	}

	s.extractAttachments("email-multi-test", attachments)

	if _, ok := storage.getExtractedText("email-multi-test", "doc.pdf"); !ok {
		t.Error("expected extraction for PDF")
	}
	if _, ok := storage.getExtractedText("email-multi-test", "photo.jpg"); ok {
		t.Error("expected no extraction for JPEG")
	}
	if _, ok := storage.getExtractedText("email-multi-test", "data.xlsx"); ok {
		t.Error("expected no extraction for XLSX without sidecar client")
	}
}
