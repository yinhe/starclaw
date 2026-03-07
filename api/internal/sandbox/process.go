package sandbox

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"
	"time"
)

// AppProcess represents a running web application in a workspace
type AppProcess struct {
	WorkspaceID string
	Port        int
	Command     string
	Cmd         *exec.Cmd
	Cancel      context.CancelFunc
	StartedAt   time.Time
	Ready       bool
}

// ProcessManager manages background web server processes per workspace
type ProcessManager struct {
	mu        sync.Mutex
	processes map[string]*AppProcess // workspace_id -> process
	nextPort  int
	minPort   int
	maxPort   int
}

// NewProcessManager creates a new process manager with a port range
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*AppProcess),
		nextPort:  9001,
		minPort:   9001,
		maxPort:   9100,
	}
}

// allocatePort finds an available port in the range
func (pm *ProcessManager) allocatePort() (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	usedPorts := make(map[int]bool)
	for _, p := range pm.processes {
		usedPorts[p.Port] = true
	}

	for port := pm.minPort; port <= pm.maxPort; port++ {
		if usedPorts[port] {
			continue
		}
		// Check if port is actually free
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", pm.minPort, pm.maxPort)
}

// StartApp starts a web application in the given workspace
func (pm *ProcessManager) StartApp(sandbox *Manager, workspaceID, command string) (*AppProcess, error) {
	// Stop existing process if any
	pm.StopApp(workspaceID)

	port, err := pm.allocatePort()
	if err != nil {
		return nil, err
	}

	ws := sandbox.GetOrCreateWorkspace(workspaceID)

	ctx, cancel := context.WithCancel(context.Background())

	// Inject PORT env var so the app knows which port to listen on
	fullCmd := fmt.Sprintf("PORT=%d %s", port, command)
	cmd := exec.CommandContext(ctx, "sh", "-c", fullCmd)
	cmd.Dir = ws.Path
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("PORT=%d", port))

	// Capture output for debugging
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start app: %v", err)
	}

	app := &AppProcess{
		WorkspaceID: workspaceID,
		Port:        port,
		Command:     command,
		Cmd:         cmd,
		Cancel:      cancel,
		StartedAt:   time.Now(),
		Ready:       false,
	}

	pm.mu.Lock()
	pm.processes[workspaceID] = app
	pm.mu.Unlock()

	// Wait for the server to become ready (poll port)
	go func() {
		for i := 0; i < 30; i++ { // wait up to 15 seconds
			time.Sleep(500 * time.Millisecond)
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
			if err == nil {
				conn.Close()
				pm.mu.Lock()
				if p, ok := pm.processes[workspaceID]; ok {
					p.Ready = true
				}
				pm.mu.Unlock()
				log.Printf("[ProcessMgr] App ready: workspace=%s port=%d", workspaceID, port)
				return
			}
		}
		log.Printf("[ProcessMgr] App failed to become ready: workspace=%s port=%d", workspaceID, port)
	}()

	// Cleanup when process exits
	go func() {
		cmd.Wait()
		pm.mu.Lock()
		if p, ok := pm.processes[workspaceID]; ok && p.Cmd == cmd {
			delete(pm.processes, workspaceID)
		}
		pm.mu.Unlock()
		log.Printf("[ProcessMgr] App exited: workspace=%s", workspaceID)
	}()

	log.Printf("[ProcessMgr] Started app: workspace=%s port=%d cmd=%s", workspaceID, port, command)
	return app, nil
}

// StopApp stops a running application in the workspace
func (pm *ProcessManager) StopApp(workspaceID string) bool {
	pm.mu.Lock()
	app, ok := pm.processes[workspaceID]
	if ok {
		delete(pm.processes, workspaceID)
	}
	pm.mu.Unlock()

	if !ok {
		return false
	}

	app.Cancel()
	// Give it a moment to shut down gracefully
	done := make(chan struct{})
	go func() {
		if app.Cmd.Process != nil {
			app.Cmd.Process.Kill()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	log.Printf("[ProcessMgr] Stopped app: workspace=%s port=%d", workspaceID, app.Port)
	return true
}

// GetApp returns the running app for a workspace, if any
func (pm *ProcessManager) GetApp(workspaceID string) *AppProcess {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.processes[workspaceID]
}

// ListApps returns all running apps
func (pm *ProcessManager) ListApps() []*AppProcess {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	apps := make([]*AppProcess, 0, len(pm.processes))
	for _, app := range pm.processes {
		apps = append(apps, app)
	}
	return apps
}
