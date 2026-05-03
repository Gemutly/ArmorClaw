package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/armorclaw/jetski/internal/approval"
	"github.com/armorclaw/jetski/internal/cdp"
	"github.com/armorclaw/jetski/internal/network"
	"github.com/armorclaw/jetski/internal/rpc"
	"github.com/armorclaw/jetski/internal/security"
	"github.com/armorclaw/jetski/internal/sonar"
	"github.com/armorclaw/jetski/internal/subprocess"
	"github.com/armorclaw/jetski/pkg/config"
	"github.com/armorclaw/jetski/pkg/logger"

	"github.com/gorilla/websocket"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "Path to configuration file")
	port       = flag.String("port", "", "CDP server port (overrides config)")
	logLevel   = flag.String("log-level", "", "Log level (DEBUG, INFO, WARN, ERROR)")
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	flag.Parse()

	log := logger.NewDefault()
	log.Info("🚀 Starting Jetski Browser")

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		log.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	if *port != "" {
		cfg.Server.Port = *port
	}

	if *logLevel != "" {
		log.SetLevel(logger.ParseLevel(*logLevel))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pi := setupProcessManager(cfg, log)
	proxyMgr := setupProxyManager(cfg, log)
	piiScanner := setupPIIScanner(cfg, log)
	router := setupMethodRouter()

	sonarBuf := sonar.NewCircularBuffer(1000)
	sonarReporter := sonar.NewReporter(sonarBuf, "wreckage", pi)
	log.Info("Sonar telemetry initialized", "buffer_capacity", 1000)
	_ = sonarReporter

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go handleShutdown(sigChan, cancel, pi, proxyMgr, log)

	engineURL := fmt.Sprintf("ws://127.0.0.1:%s", cfg.Browser.EnginePort)
	cdpProxy := cdp.NewProxy(engineURL, router, piiScanner, cfg.Security.EncryptSession)
	cdpProxy.SetRecorder(func(method string, params json.RawMessage) {
		sonar.RecordFrame(sonarBuf, method, params, cdpProxy.GetSessionID())
	})

	var approvalClient *approval.ApprovalClient
	if cfg.Approval.Enabled {
		approvalClient = approval.NewApprovalClient(cfg.Approval.BridgeURL, cfg.Approval.RoomID, cfg.Approval.Timeout)
		cdpProxy.SetApprovalClient(approvalClient)
		cdpProxy.SetApprovalTimeout(cfg.Approval.Timeout)
		log.Info("Approval client wired", "bridge_url", cfg.Approval.BridgeURL, "room_id", cfg.Approval.RoomID, "timeout", cfg.Approval.Timeout)
	}

	eventEmitter := cdp.NewEventEmitter(cfg.Security.EmitStateEvents)
	if eventEmitter.Enabled() {
		cdpProxy.SetEventEmitter(eventEmitter)
		log.Info("Event emitter enabled, CDP events will stream to subscribers")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("WebSocket upgrade failed", "error", err)
			return
		}

		if err := cdpProxy.Start(conn); err != nil {
			log.Error("Failed to start CDP proxy", "error", err)
			conn.Close()
			return
		}

		go func() {
			<-ctx.Done()
			cdpProxy.Stop()
		}()
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","port":"%s"}`, cfg.Server.Port)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	log.Info("Starting CDP proxy server", "port", cfg.Server.Port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server error", "error", err)
		}
	}()

	rpcServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, "9223"),
		Handler:      rpc.NewServer(approvalClient, eventEmitter).Handler(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	log.Info("Starting RPC API server", "port", "9223")

	go func() {
		if err := rpcServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("RPC server error", "error", err)
		}
	}()

	log.Info("✅ Jetski Browser started successfully", "cdp_port", cfg.Server.Port, "rpc_port", "9223")
	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	_ = server.Shutdown(shutdownCtx)
	_ = rpcServer.Shutdown(shutdownCtx)

	log.Info("✅ Jetski Browser stopped gracefully")
}

func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config.LoadDefault()
	}
	return config.Load(path)
}

func setupProcessManager(cfg *config.Config, log *logger.Logger) *subprocess.ProcessManager {
	pi := subprocess.NewProcessManager()

	if err := pi.StartWithSupervisor(context.Background(), cfg.Browser.EnginePort); err != nil {
		log.Error("Failed to start browser engine", "error", err)
		os.Exit(1)
	}

	log.Info("Browser engine started", "port", cfg.Browser.EnginePort)
	return pi
}

func setupProxyManager(cfg *config.Config, log *logger.Logger) *network.ProxyManager {
	if !cfg.Network.ProxyEnabled || len(cfg.Network.ProxyList) == 0 {
		return nil
	}

	proxyConfig := network.ProxyManagerConfig{
		ProxyList:      cfg.Network.ProxyList,
		HealthCheckURL: cfg.Network.ProxyHealthCheckURL,
		HealthInterval: cfg.Network.ProxyHealthInterval,
		RequestTimeout: 30 * time.Second,
	}

	proxyMgr, err := network.NewProxyManager(proxyConfig)
	if err != nil {
		log.Error("Failed to initialize proxy manager", "error", err)
		os.Exit(1)
	}

	proxyMgr.StartHealthChecks(context.Background())
	log.Info("Proxy manager initialized", "proxies", len(cfg.Network.ProxyList))
	return proxyMgr
}

func setupPIIScanner(cfg *config.Config, log *logger.Logger) *security.PIIScanner {
	if !cfg.Security.PIIScanning {
		return nil
	}

	scanner := security.NewPIIScanner()
	log.Info("PII scanner enabled")
	return scanner
}

func setupMethodRouter() *cdp.MethodRouter {
	router := cdp.NewMethodRouter(cdp.NewTranslator())
	return router
}

func handleShutdown(
	sigChan chan os.Signal,
	cancel context.CancelFunc,
	pi *subprocess.ProcessManager,
	proxyMgr *network.ProxyManager,
	log *logger.Logger,
) {
	sig := <-sigChan
	log.Info("Received shutdown signal", "signal", sig.String())

	log.Info("Initiating graceful shutdown...")

	if proxyMgr != nil {
		log.Info("Stopping proxy manager")
		proxyMgr.StopHealthChecks()
	}

	cancel()

	log.Info("✅ Shutdown complete")
}
