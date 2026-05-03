package rpc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armorclaw/jetski/internal/approval"
	"github.com/armorclaw/jetski/internal/cdp"
)

type Server struct {
	startTime    time.Time
	sessions     map[string]struct{}
	mu           sync.RWMutex
	counter      atomic.Int64
	ac           *approval.ApprovalClient
	eventEmitter *cdp.EventEmitter
}

func NewServer(ac *approval.ApprovalClient, emitter *cdp.EventEmitter) *Server {
	return &Server{
		startTime:    time.Now(),
		sessions:     make(map[string]struct{}),
		ac:           ac,
		eventEmitter: emitter,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/status", s.handleStatus)
	mux.HandleFunc("/rpc/session/create", s.handleSessionCreate)
	mux.HandleFunc("/rpc/session/close", s.handleSessionClose)
	mux.HandleFunc("/rpc/health", s.handleHealth)
	mux.HandleFunc("/rpc/events.subscribe", s.handleEventsSubscribe)
	if s.ac != nil {
		approval.RegisterApprovalHandlers(mux, s.ac)
	}
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	active := len(s.sessions)
	s.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"active_sessions": active,
		"engine_health":   "ok",
	})
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := fmt.Sprintf("session-%d", s.counter.Add(1))

	s.mu.Lock()
	s.sessions[id] = struct{}{}
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"id": id,
	})
}

func (s *Server) handleSessionClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	s.mu.Lock()
	_, exists := s.sessions[req.ID]
	if !exists {
		s.mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "session not found"})
		return
	}
	delete(s.sessions, req.ID)
	s.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Seconds()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"uptime": uptime,
	})
}

type subscribeRequest struct {
	Type    string `json:"type"`
	Payload struct {
		DeviceID string `json:"device_id"`
	} `json:"payload"`
}

func (s *Server) handleEventsSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.eventEmitter == nil || !s.eventEmitter.Enabled() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "event streaming is disabled: set emit_state_events=true",
		})
		return
	}

	if r.Method == http.MethodPost {
		var req subscribeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
			return
		}
		if req.Type != "register" || req.Payload.DeviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "requires {type:register, payload:{device_id:...}}"})
			return
		}

		ch, err := s.eventEmitter.Subscribe(req.Payload.DeviceID)
		if err != nil {
			if err == cdp.ErrAlreadySubscribed {
				w.WriteHeader(http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		flusher, canFlush := w.(http.Flusher)
		_ = canFlush

		fmt.Fprintf(w, "event: registered\ndata: {\"device_id\":\"%s\"}\n\n", req.Payload.DeviceID)
		if canFlush {
			flusher.Flush()
		}

		ctx := r.Context()
		defer s.eventEmitter.Unsubscribe(req.Payload.DeviceID)

		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(evt)
				fmt.Fprintf(w, "event: cdp\ndata: %s\n\n", data)
				if canFlush {
					flusher.Flush()
				}
			}
		}
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "use POST with registration handshake"})
	log.Printf("[JETSKI RPC]: events.subscribe invalid method %s from %s", r.Method, r.RemoteAddr)
}
