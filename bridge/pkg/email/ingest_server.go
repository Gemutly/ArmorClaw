package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/armorclaw/bridge/pkg/eventbus"
	"github.com/armorclaw/bridge/pkg/logger"
	"github.com/armorclaw/bridge/pkg/pii"
	"github.com/armorclaw/bridge/pkg/sidecar"
	"github.com/armorclaw/bridge/pkg/yara"
)

type IngestServer struct {
	storage      EmailStorage
	bus          *eventbus.EventBus
	masker       *pii.Masker
	log          *logger.Logger
	yaraScan     func(filePath string) (bool, error)
	socket       string
	listener     net.Listener
	officeClient *sidecar.Client
	rustClient   *sidecar.Client
	javaClient   *sidecar.Client
}

type IngestServerConfig struct {
	Storage             EmailStorage
	Bus                 *eventbus.EventBus
	Socket              string
	Log                 *logger.Logger
	SidecarOfficeClient *sidecar.Client
	SidecarRustClient   *sidecar.Client
	SidecarJavaClient   *sidecar.Client
}

func NewIngestServer(cfg IngestServerConfig) *IngestServer {
	s := &IngestServer{
		storage:      cfg.Storage,
		bus:          cfg.Bus,
		masker:       pii.NewMasker(),
		socket:       cfg.Socket,
		log:          cfg.Log,
		officeClient: cfg.SidecarOfficeClient,
		rustClient:   cfg.SidecarRustClient,
		javaClient:   cfg.SidecarJavaClient,
	}
	s.yaraScan = s.defaultYARAScan
	if s.socket == "" {
		s.socket = "/run/armorclaw/email-ingest.sock"
	}
	return s
}

func (s *IngestServer) Start() error {
	var err error
	if err := os.MkdirAll(filepath.Dir(s.socket), 0755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	s.listener, err = net.Listen("unix", s.socket)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.socket, err)
	}
	go s.acceptLoop()
	s.log.Info("ingest_server_started", "socket", s.socket)
	return nil
}

func (s *IngestServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *IngestServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *IngestServer) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	line, err := readLine(conn)
	if err != nil {
		return
	}

	var header struct {
		Action  string `json:"action"`
		From    string `json:"from"`
		To      string `json:"to"`
		QueueID string `json:"queue_id"`
		DataLen int    `json:"data_len"`
	}
	if err := json.Unmarshal(line, &header); err != nil {
		return
	}

	if header.Action != "ingest" {
		return
	}

	rawEmail := make([]byte, header.DataLen)
	if _, err := io.ReadFull(conn, rawEmail); err != nil {
		return
	}

	resp := s.IngestEmail(context.Background(), rawEmail, header.From, header.To, header.QueueID)
	respJSON, _ := json.Marshal(resp)
	conn.Write(respJSON)
}

func (s *IngestServer) IngestEmail(ctx context.Context, rawEmail []byte, from, to, queueID string) *IngestResponse {
	emailID := generateEmailID(from, to, queueID)

	parsed, err := ParseMIME(rawEmail)
	if err != nil {
		s.log.Error("ingest_parse_failed", "email_id", emailID, "error", err)
		return &IngestResponse{Accepted: false, RejectionReason: "invalid MIME"}
	}

	if err := s.storage.StoreEmail(emailID, rawEmail); err != nil {
		s.log.Error("ingest_store_failed", "email_id", emailID, "error", err)
		return &IngestResponse{Accepted: false, RejectionReason: "storage error"}
	}

	var fileIDs []string
	for _, att := range parsed.Attachments {
		fid, err := s.storage.StoreAttachment(emailID, att.Filename, att.Content)
		if err != nil {
			s.log.Error("ingest_attachment_store_failed", "email_id", emailID, "filename", att.Filename, "error", err)
			continue
		}
		fileIDs = append(fileIDs, fid)
	}

	yaraOK := true
	if s.yaraScan != nil {
		for _, att := range parsed.Attachments {
			tmpPath := fmt.Sprintf("/tmp/armorclaw-scan-%s-%s", emailID, att.Filename)
			isClean, err := s.yaraScan(tmpPath)
			if err != nil {
				s.log.Warn("yara_scan_error", "email_id", emailID, "error", err)
			}
			if !isClean {
				yaraOK = false
				s.log.Warn("yara_malware_detected", "email_id", emailID, "filename", att.Filename)
			}
		}
	}

	if !yaraOK {
		return &IngestResponse{Accepted: false, RejectionReason: "malware detected"}
	}

	if yaraOK && (s.officeClient != nil || s.rustClient != nil) {
		go s.extractAttachments(emailID, parsed.Attachments)
	}

	bodyMasked, maskedFields := s.masker.MaskPII(parsed.BodyText)
	subjectMasked, _ := s.masker.MaskPII(parsed.Subject)

	var piiTypes []string
	seen := map[string]bool{}
	for _, f := range maskedFields {
		if !seen[f.Type] {
			piiTypes = append(piiTypes, f.Type)
			seen[f.Type] = true
		}
	}

	evt := NewEmailReceivedEvent(
		from,
		to,
		subjectMasked,
		bodyMasked,
		emailID,
		fileIDs,
		piiTypes,
	)
	s.bus.PublishBridgeEvent(evt)

	s.log.Info("email_ingested", "email_id", emailID, "from", from, "to", to, "attachments", len(parsed.Attachments), "pii_fields", len(piiTypes))

	return &IngestResponse{
		Accepted: true,
		EmailID:  emailID,
		FileIDs:  fileIDs,
	}
}

var mimeToFormat = map[string]string{
	"application/pdf":                                                          "pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":  "docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":        "xlsx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/vnd.ms-excel":                                                 "xls",
	"application/vnd.ms-powerpoint":                                            "ppt",
	"application/msword":                                                       "doc",
	"application/vnd.ms-outlook":                                               "msg",
}

const maxExtractionSize = 10 * 1024 * 1024 // 10MB

func (s *IngestServer) extractAttachments(emailID string, attachments []ParsedAttachment) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for _, att := range attachments {
		if att.Size > maxExtractionSize {
			s.log.Debug("attachment_extraction_skipped_oversized",
				"email_id", emailID,
				"filename", att.Filename,
				"size", att.Size,
				"max", maxExtractionSize,
			)
			continue
		}

		format, ok := mimeToFormat[att.ContentType]
		if !ok {
			s.log.Debug("attachment_extraction_skipped_format",
				"email_id", emailID,
				"filename", att.Filename,
				"content_type", att.ContentType,
			)
			continue
		}

		req := &sidecar.ExtractTextRequest{
			DocumentFormat:  format,
			DocumentContent: att.Content,
		}

		resp, err := sidecar.RouteExtractText(ctx, req, s.officeClient, s.rustClient, s.javaClient)
		if err != nil {
			s.log.Warn("attachment_extraction_failed",
				"email_id", emailID,
				"filename", att.Filename,
				"format", format,
				"error", err,
			)
			continue
		}

		if resp != nil && resp.Text != "" {
			if err := s.storage.StoreAttachmentText(emailID, att.Filename, resp.Text); err != nil {
				s.log.Warn("attachment_text_store_failed",
					"email_id", emailID,
					"filename", att.Filename,
					"error", err,
				)
			} else {
				s.log.Info("attachment_text_extracted",
					"email_id", emailID,
					"filename", att.Filename,
					"format", format,
					"text_len", len(resp.Text),
				)
			}
		}
	}
}

func (s *IngestServer) defaultYARAScan(filePath string) (bool, error) {
	return yara.ScanFileForMalware(filePath)
}

type IngestResponse struct {
	Accepted        bool     `json:"accepted"`
	EmailID         string   `json:"email_id"`
	FileIDs         []string `json:"file_ids"`
	RejectionReason string   `json:"rejection_reason,omitempty"`
}

func generateEmailID(from, to, queueID string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d", from, to, queueID, time.Now().UnixNano())))
	return hex.EncodeToString(h[:16])
}

func readLine(r io.Reader) ([]byte, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		_, err := r.Read(b)
		if err != nil {
			return buf, err
		}
		if b[0] == '\n' {
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) > 4096 {
			return nil, fmt.Errorf("header too large")
		}
	}
}
