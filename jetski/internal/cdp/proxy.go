package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armorclaw/jetski/internal/approval"
	"github.com/armorclaw/jetski/internal/security"
	"github.com/gorilla/websocket"
)

const (
	PingInterval   = 30 * time.Second
	PongTimeout    = 60 * time.Second
	WriteWait      = 10 * time.Second
	ReadWait       = PongTimeout
	MaxMessageSize = 10 * 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type CDPMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *CDPError       `json:"error,omitempty"`
}

type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

type MessageRecorder func(method string, params json.RawMessage)

type PIIScanner interface {
	ScanJSONMessage(jsonStr string) ([]security.PIIFinding, error)
}

type ApprovalChecker interface {
	RequestApproval(ctx context.Context, op approval.OperationType, detail string) (bool, error)
}

type Proxy struct {
	mu              sync.Mutex
	clientConn      *websocket.Conn
	engineConn      *websocket.Conn
	engineURL       string
	ctx             context.Context
	cancel          context.CancelFunc
	router          *MethodRouter
	errorChan       chan error
	recorder        MessageRecorder
	eventEmitter    *EventEmitter
	piiScanner      PIIScanner
	tetheredMode    bool
	approvalClient  ApprovalChecker
	approvalTimeout time.Duration
	sessionID       atomic.Value
}

func NewProxy(engineURL string, router *MethodRouter, piiScanner PIIScanner, tetheredMode bool) *Proxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &Proxy{
		engineURL:    engineURL,
		ctx:          ctx,
		cancel:       cancel,
		router:       router,
		errorChan:    make(chan error, 10),
		piiScanner:   piiScanner,
		tetheredMode: tetheredMode,
	}
}

func (p *Proxy) SetRecorder(r MessageRecorder) {
	p.recorder = r
}

func (p *Proxy) SetEventEmitter(em *EventEmitter) {
	p.eventEmitter = em
}

func (p *Proxy) SetApprovalClient(client ApprovalChecker) {
	p.approvalClient = client
}

func (p *Proxy) SetApprovalTimeout(timeout time.Duration) {
	p.approvalTimeout = timeout
}

func (p *Proxy) SetSessionID(id string) {
	p.sessionID.Store(id)
}

func (p *Proxy) GetSessionID() string {
	v := p.sessionID.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (p *Proxy) extractSessionID(data []byte) {
	var msg struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(data, &msg) != nil || msg.Result == nil {
		return
	}
	var result struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(msg.Result, &result) == nil && result.SessionID != "" {
		p.SetSessionID(result.SessionID)
	}
}

func (p *Proxy) Start(clientConn *websocket.Conn) error {
	p.mu.Lock()
	if p.clientConn != nil {
		p.mu.Unlock()
		return errors.New("proxy already started")
	}
	p.clientConn = clientConn
	p.mu.Unlock()

	if err := p.connectToEngine(); err != nil {
		return err
	}

	p.clientConn.SetReadLimit(MaxMessageSize)
	p.clientConn.SetReadDeadline(time.Now().Add(ReadWait))
	p.clientConn.SetPongHandler(func(string) error {
		p.clientConn.SetReadDeadline(time.Now().Add(ReadWait))
		return nil
	})

	go p.forwardToEngine()
	go p.forwardToClient()
	go p.pingLoop()

	return nil
}

func (p *Proxy) connectToEngine() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(p.engineURL, nil)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.engineConn = conn
	p.mu.Unlock()

	p.engineConn.SetReadLimit(MaxMessageSize)
	p.engineConn.SetReadDeadline(time.Now().Add(ReadWait))
	p.engineConn.SetPongHandler(func(string) error {
		p.engineConn.SetReadDeadline(time.Now().Add(ReadWait))
		return nil
	})

	return nil
}

func (p *Proxy) needsApproval(msg *CDPMessage) (bool, approval.OperationType, string) {
	if p.approvalClient == nil {
		return false, "", ""
	}

	switch msg.Method {
	case "Input.insertText":
		var params struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil || params.Text == "" {
			return false, "", ""
		}
		piiTypes := p.detectPIITypes(params.Text)
		if len(piiTypes) > 0 {
			return true, approval.OpPIIInput, "PII detected: " + strings.Join(piiTypes, ", ")
		}
		return false, "", ""

	case "Page.navigate":
		var params struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil || params.URL == "" {
			return false, "", ""
		}
		return true, approval.OpNavigation, "Navigate to: " + params.URL

	default:
		return false, "", ""
	}
}

func (p *Proxy) detectPIITypes(text string) []string {
	var found []string
	for piiType, pattern := range scrubPatterns {
		if pattern.MatchString(text) {
			found = append(found, piiType)
		}
	}
	return found
}

func (p *Proxy) checkApproval(msg *CDPMessage) error {
	needed, opType, detail := p.needsApproval(msg)
	if !needed {
		return nil
	}

	timeout := p.approvalTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()

	approved, err := p.approvalClient.RequestApproval(ctx, opType, detail)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return errors.New("approval request timed out")
		}
		return err
	}
	if !approved {
		return errors.New("approval denied for " + string(opType))
	}
	return nil
}

func (p *Proxy) forwardToEngine() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[JETSKI PROXY]: Panic in forwardToEngine: %v", r)
		}
	}()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			messageType, data, err := p.clientConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					select {
					case p.errorChan <- err:
					case <-p.ctx.Done():
					}
				}
				return
			}

			var msg CDPMessage
			if messageType == websocket.TextMessage {
				if err := json.Unmarshal(data, &msg); err != nil {
					log.Printf("[JETSKI PROXY]: Failed to unmarshal message: %v", err)
					continue
				}

				data = p.ScrubPII(data)

				if p.piiScanner != nil {
					if findings, err := p.piiScanner.ScanJSONMessage(string(data)); err == nil {
						for _, f := range findings {
							log.Printf("[JETSKI PII] detected %s in CDP message", f.Type)
						}
					}
				}

				if p.recorder != nil && msg.Method != "" {
					p.recorder(msg.Method, msg.Params)
				}

				if p.eventEmitter != nil && msg.Method != "" {
					p.eventEmitter.Emit(msg.Method, msg.Params)
				}

				if msg.Method != "" {
					route := p.router.Route(msg.Method)
					if route != nil && route.Handler != nil {
						translated, err := route.Handler(&msg)
						if err != nil {
							p.sendError(msg.ID, err)
							continue
						}
						msg = *translated
					}

					if err := p.checkApproval(&msg); err != nil {
						p.sendError(msg.ID, err)
						continue
					}
				}
			}

			if err := p.writeMessage(p.engineConn, messageType, data); err != nil {
				select {
				case p.errorChan <- err:
				case <-p.ctx.Done():
				}
				return
			}
		}
	}
}

func (p *Proxy) forwardToClient() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[JETSKI PROXY]: Panic in forwardToClient: %v", r)
		}
	}()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			messageType, data, err := p.engineConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					select {
					case p.errorChan <- err:
					case <-p.ctx.Done():
					}
				}
				return
			}

			if p.recorder != nil && messageType == websocket.TextMessage {
				var msg CDPMessage
				if json.Unmarshal(data, &msg) == nil && msg.Method != "" {
					p.recorder(msg.Method, msg.Params)
				}
			}

			if p.eventEmitter != nil && messageType == websocket.TextMessage {
				var msg CDPMessage
				if json.Unmarshal(data, &msg) == nil && msg.Method != "" {
					p.eventEmitter.Emit(msg.Method, msg.Params)
				}
			}

			if messageType == websocket.TextMessage {
				p.extractSessionID(data)
			}

			if err := p.writeMessage(p.clientConn, messageType, data); err != nil {
				select {
				case p.errorChan <- err:
				case <-p.ctx.Done():
				}
				return
			}
		}
	}
}

func (p *Proxy) writeMessage(conn *websocket.Conn, messageType int, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn.SetWriteDeadline(time.Now().Add(WriteWait))
	return conn.WriteMessage(messageType, data)
}

func (p *Proxy) pingLoop() {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.pingConnections(); err != nil {
				p.errorChan <- err
				return
			}
		}
	}
}

func (p *Proxy) pingConnections() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.clientConn != nil {
		p.clientConn.SetWriteDeadline(time.Now().Add(WriteWait))
		if err := p.clientConn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return err
		}
	}

	if p.engineConn != nil {
		p.engineConn.SetWriteDeadline(time.Now().Add(WriteWait))
		if err := p.engineConn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return err
		}
	}

	return nil
}

func (p *Proxy) sendError(id int, err error) {
	errorMsg := CDPMessage{
		ID: id,
		Error: &CDPError{
			Code:    -32000,
			Message: err.Error(),
		},
	}

	data, _ := json.Marshal(errorMsg)
	p.writeMessage(p.clientConn, websocket.TextMessage, data)
}

func (p *Proxy) Stop() {
	p.cancel()

	p.mu.Lock()
	if p.clientConn != nil {
		_ = p.clientConn.Close()
		p.clientConn = nil
	}
	if p.engineConn != nil {
		_ = p.engineConn.Close()
		p.engineConn = nil
	}
	p.mu.Unlock()

	select {
	case <-p.errorChan:
	default:
	}
	close(p.errorChan)
}

func (p *Proxy) Errors() <-chan error {
	return p.errorChan
}

func ScrubPII(data []byte, patterns map[string]*regexp.Regexp) []byte {
	result := data
	for piiType, pattern := range patterns {
		replacement := "[REDACTED_" + piiType + "]"
		result = pattern.ReplaceAll(result, []byte(replacement))
	}
	return result
}

func (p *Proxy) ScrubPII(data []byte) []byte {
	if !p.tetheredMode {
		return data
	}
	return ScrubPII(data, scrubPatterns)
}

var scrubPatterns map[string]*regexp.Regexp

func init() {
	scrubPatterns = map[string]*regexp.Regexp{
		"SSN":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		"CREDIT_CARD": regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		"EMAIL":       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		"PASSWORD":    regexp.MustCompile(`(?i)(password|passwd|pwd)["\']?\s*[:=]\s*["\']?[^\s"']{8,}`),
	}
}
