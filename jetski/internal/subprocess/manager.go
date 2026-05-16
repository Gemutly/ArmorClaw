package subprocess

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type ProcessManager struct {
	mu           sync.Mutex
	cmd          *exec.Cmd
	healthTicker *time.Ticker
	failures     int
	maxFailures  int
	enginePath   string
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		maxFailures: 3,
	}
}

func (pm *ProcessManager) StartWithSupervisor(ctx context.Context, enginePath string, port string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.enginePath = enginePath
	pm.cmd = exec.CommandContext(ctx, enginePath, "--port="+port)

	pm.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := pm.cmd.Start(); err != nil {
		return err
	}

	log.Printf("[JETSKI SUPERVISOR]: Engine started with PID %d", pm.cmd.Process.Pid)

	go pm.watchdogLoop(ctx)

	return nil
}

func (pm *ProcessManager) killProcessGroup() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.cmd == nil || pm.cmd.Process == nil {
		return errors.New("no process running")
	}

	pgid, err := syscall.Getpgid(pm.cmd.Process.Pid)
	if err == nil {
		log.Printf("[JETSKI SUPERVISOR]: Reaping zombie process group %d", pgid)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	return pm.cmd.Wait()
}

func (pm *ProcessManager) watchdogLoop(ctx context.Context) {
	pm.healthTicker = time.NewTicker(5 * time.Second)
	defer pm.healthTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = pm.killProcessGroup()
			return
		case <-pm.healthTicker.C:
			if !pm.checkHealth() {
				pm.failures++
				log.Printf("[JETSKI SUPERVISOR]: Health check failed (%d/%d)", pm.failures, pm.maxFailures)

				if pm.failures >= pm.maxFailures {
					log.Println("[JETSKI SUPERVISOR]: Circuit breaker tripped. Initiating restart...")
					pm.restartEngine(ctx)
				}
			} else {
				pm.failures = 0
			}
		}
	}
}

func (pm *ProcessManager) checkHealth() bool {
	return true
}

func (pm *ProcessManager) restartEngine(ctx context.Context) {
	_ = pm.killProcessGroup()
	_ = pm.StartWithSupervisor(ctx, pm.enginePath, "9223")
}
