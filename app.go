package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// App is the main Wails application struct.
type App struct {
	ctx            context.Context
	mu             sync.Mutex
	status         string
	conn           *SecureConn
	proxy          *LeagueProxy
	license        *LoginOKPayload
	stopCh         chan struct{}
	licenseCheckCh chan struct{}
	debugLog       []string
	debugMu        sync.Mutex

	stopOnce    sync.Once
	licenseOnce sync.Once
	cleanupOnce sync.Once
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{
		status: "disconnected",
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	terminateRiotServices()
}

func (a *App) shutdown(ctx context.Context) {
	a.cleanup()
}

// --- Wails bindings: common ---

// GetPlatform returns "windows" or "darwin".
func (a *App) GetPlatform() string {
	return runtime.GOOS
}

// GetStatus returns the current app status.
func (a *App) GetStatus() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// GetLicenseInfo returns license details after login.
func (a *App) GetLicenseInfo() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.license == nil {
		return nil
	}
	return map[string]interface{}{
		"has_license": a.license.HasLicense,
		"expires_at":  a.license.ExpiresAt,
	}
}

// Login connects to the server via encrypted TCP and authenticates.
func (a *App) Login(username, password string) string {
	a.mu.Lock()
	if a.status != "disconnected" {
		a.mu.Unlock()
		return "already connected"
	}
	a.status = "connecting"
	a.mu.Unlock()

	conn, err := Connect()
	if err != nil {
		a.mu.Lock()
		a.status = "disconnected"
		a.mu.Unlock()
		return "connection failed"
	}

	license, err := conn.Login(username, password)
	if err != nil {
		_ = conn.Close()
		a.mu.Lock()
		a.status = "disconnected"
		a.mu.Unlock()
		msg := err.Error()
		switch msg {
		case "invalid credentials", "too many devices", "no active license":
			return msg
		default:
			return "connection failed"
		}
	}

	a.mu.Lock()
	a.conn = conn
	a.license = license
	a.status = "logged_in"
	a.licenseCheckCh = make(chan struct{})
	a.mu.Unlock()

	go a.licenseCheckLoop()

	return "ok"
}

// Logout disconnects from the server.
func (a *App) Logout() string {
	a.cleanup()
	return "ok"
}

// --- Windows mode: client ---

// PrepareLogin starts the Riot Client for the user to log in manually (Windows only).
// Starts the proxy and launches RCS — user logs in themselves, no YAML capture needed.
func (a *App) PrepareLogin(rcsPath string) string {
	a.mu.Lock()
	if a.status != "logged_in" {
		a.mu.Unlock()
		return "must be logged in first"
	}
	if a.license == nil || !a.license.HasLicense {
		a.mu.Unlock()
		return "no active license"
	}
	a.status = "login_phase"
	a.mu.Unlock()

	a.logDebug("[main-pc] PrepareLogin started, rcsPath=%q, platform=%s", rcsPath, runtime.GOOS)

	if errMsg := a.heartbeatOrFail(); errMsg != "" {
		a.logDebug("heartbeat failed: %s", errMsg)
		a.flushLogs()
		return errMsg
	}
	a.logDebug("heartbeat ok")

	terminateRiotServices()
	a.logDebug("terminated riot services")

	if IsVanguardRunning() {
		StopVanguard()
		a.logDebug("stopped vanguard")
	}

	proxy, err := NewLeagueProxy()
	if err != nil {
		a.logDebug("proxy create failed: %v", err)
		a.flushLogs()
		a.mu.Lock()
		a.status = "logged_in"
		a.mu.Unlock()
		return "setup failed"
	}

	port, err := proxy.Start()
	if err != nil {
		a.logDebug("proxy start failed: %v", err)
		a.flushLogs()
		a.mu.Lock()
		a.status = "logged_in"
		a.mu.Unlock()
		return "setup failed"
	}
	a.proxy = proxy
	a.logDebug("proxy started on port %d", port)

	rcs := rcsPath
	if rcs == "" {
		rcs = FindRiotClientServices()
		if rcs == "" {
			a.logDebug("RCS not found")
			a.flushLogs()
			_ = proxy.Stop()
			a.mu.Lock()
			a.status = "logged_in"
			a.mu.Unlock()
			return "could not find client executable"
		}
	}
	a.logDebug("RCS found: %s", rcs)

	if err := LaunchRCSLogin(rcs, port, decodeStr(patchlineLiveObf)); err != nil {
		a.logDebug("RCS launch failed: %v", err)
		a.flushLogs()
		_ = proxy.Stop()
		a.mu.Lock()
		a.status = "logged_in"
		a.mu.Unlock()
		return "launch failed"
	}
	a.logDebug("RCS launched — user logs in manually")

	a.stopCh = make(chan struct{})
	a.mu.Lock()
	a.status = "running"
	a.mu.Unlock()

	go a.logCleanLoop()

	a.flushLogs()
	return "ok"
}

// Stop cancels any in-progress operation (Windows only).
func (a *App) Stop() string {
	a.mu.Lock()
	s := a.status
	a.mu.Unlock()

	if s == "disconnected" || s == "logged_in" {
		return "nothing to stop"
	}

	a.closeStopCh()

	if a.proxy != nil {
		a.proxy.Stop()
		a.proxy = nil
	}

	a.mu.Lock()
	a.status = "logged_in"
	a.mu.Unlock()

	return "ok"
}

// --- Secondary PC mode: guardian ---

// StartWorker begins monitoring for League game on the secondary PC and kills it if it starts.
func (a *App) StartWorker() string {
	a.mu.Lock()
	if a.status != "logged_in" {
		a.mu.Unlock()
		return "must be logged in first"
	}
	if a.license == nil || !a.license.HasLicense {
		a.mu.Unlock()
		return "no active license"
	}
	a.status = "monitoring"
	a.stopCh = make(chan struct{})
	a.mu.Unlock()

	if errMsg := a.heartbeatOrFail(); errMsg != "" {
		return errMsg
	}

	a.logDebug("[2nd-pc] StartWorker, platform=%s", runtime.GOOS)
	go a.workerLoop()
	return "ok"
}

// workerLoop runs on the secondary PC:
//   - kills League game (League of Legends.exe) if it starts
//   - keeps League client alive — restarts it if it crashes
func (a *App) workerLoop() {
	a.logDebug("[2nd-pc] workerLoop started")

	if !isLeagueClientRunning() {
		a.logDebug("[2nd-pc] League client not running, launching")
		launchLeague()
		sleepOrStop(a.stopCh, 5000)
	}

	failCount := 0
	for {
		select {
		case <-a.stopCh:
			a.logDebug("[2nd-pc] workerLoop stopped")
			a.flushLogs()
			return
		default:
		}

		if isLeagueGameRunning() {
			a.logDebug("[2nd-pc] League game detected, terminating")
			killLeagueGame()
		}

		if !isLeagueClientRunning() {
			a.logDebug("[2nd-pc] League client not running, restarting")
			if err := launchLeague(); err != nil {
				failCount++
				a.logDebug("[2nd-pc] client launch failed (attempt %d): %v", failCount, err)
				backoff := 5000
				if failCount > 5 {
					backoff = 30000
				}
				sleepOrStop(a.stopCh, backoff)
				continue
			}
			failCount = 0
			sleepOrStop(a.stopCh, 5000)
			continue
		}

		failCount = 0
		sleepOrStop(a.stopCh, 1000)
	}
}

// StopWorker stops the worker monitoring loop.
func (a *App) StopWorker() string {
	a.mu.Lock()
	if a.status != "monitoring" {
		a.mu.Unlock()
		return "worker not running"
	}
	a.mu.Unlock()

	a.closeStopCh()

	a.mu.Lock()
	a.status = "logged_in"
	a.mu.Unlock()

	return "ok"
}

func (a *App) closeStopCh() {
	if a.stopCh == nil {
		return
	}
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
}

func (a *App) closeLicenseCheckCh() {
	if a.licenseCheckCh == nil {
		return
	}
	a.licenseOnce.Do(func() {
		close(a.licenseCheckCh)
	})
}

// --- internal helpers ---

// heartbeatOrFail verifies the server is real and license is still valid.
func (a *App) heartbeatOrFail() string {
	if a.conn == nil {
		return "not connected"
	}
	if err := a.conn.Heartbeat(); err != nil {
		if a.conn != nil {
			_ = a.conn.Close()
			a.conn = nil
		}
		a.mu.Lock()
		a.license = nil
		a.status = "disconnected"
		a.mu.Unlock()
		return "session verification failed"
	}
	return ""
}

// licenseCheckLoop polls the server every 5 seconds to verify the license is still valid.
// If the license is expired or revoked, it cleans up and exits the app.
func (a *App) licenseCheckLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.licenseCheckCh:
			return
		case <-ticker.C:
			a.mu.Lock()
			conn := a.conn
			a.mu.Unlock()

			if conn == nil {
				return
			}

			status, err := conn.LicenseCheck()
			if err != nil || status == nil || !status.Valid {
				a.cleanup()
				return
			}
		}
	}
}

func (a *App) cleanup() {
	a.cleanupOnce.Do(func() {
		if a.licenseCheckCh != nil {
			a.closeLicenseCheckCh()
			a.licenseCheckCh = nil
		}

		if a.stopCh != nil {
			a.closeStopCh()
		}

		if a.proxy != nil {
			a.proxy.Stop()
			a.proxy = nil
		}

		if a.conn != nil {
			_ = a.conn.Close()
			a.conn = nil
		}

		a.mu.Lock()
		a.license = nil
		a.status = "disconnected"
		a.mu.Unlock()
	})
}

// logCleanLoop removes Riot log directories every 5 seconds while the main PC is running.
func (a *App) logCleanLoop() {
	for {
		sleepOrStop(a.stopCh, 5000)
		select {
		case <-a.stopCh:
			return
		default:
			clearRiotLogs()
		}
	}
}

// FindRCS auto-detects RCS path (exposed to frontend).
func (a *App) FindRCS() string {
	return FindRiotClientServices()
}

func sleepOrStop(stopCh chan struct{}, ms int) {
	select {
	case <-stopCh:
	case <-time.After(time.Duration(ms) * time.Millisecond):
	}
}

func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func (a *App) logDebug(format string, args ...interface{}) {
	line := time.Now().Format("15:04:05.000") + " " + fmt.Sprintf(format, args...)
	a.debugMu.Lock()
	a.debugLog = append(a.debugLog, line)
	a.debugMu.Unlock()
}

func (a *App) flushLogs() {
	a.debugMu.Lock()
	lines := a.debugLog
	a.debugLog = nil
	a.debugMu.Unlock()

	if len(lines) == 0 {
		return
	}
	if a.conn != nil {
		_ = a.conn.UploadLogs(lines)
	}
}

// FindRiotClientServices is defined per-platform in launcher_*.go
// terminateRiotServices is defined per-platform in process_*.go
// isLeagueGameRunning is defined per-platform in process_*.go
// killLeagueGame is defined per-platform in process_*.go
// StopVanguard, IsVanguardRunning are in vanguard_*.go

