//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func hideCmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd
}

func getRiotProcesses() []string {
	return []string{
		decodeStr(rcsExeObf),
		decodeStr(rcuExeObf),
		decodeStr(rcuRenderExeObf),
		decodeStr(lcExeObf),
		decodeStr(lcuExeObf),
		decodeStr(lcuRenderExeObf),
	}
}

func isProcessRunning(name string) bool {
	cmd := hideCmd(exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", name), "/NH"))
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(name))
}

func terminateRiotServices() {
	for _, proc := range getRiotProcesses() {
		if isProcessRunning(proc) {
			cmd := hideCmd(exec.Command("taskkill", "/F", "/IM", proc))
			cmd.CombinedOutput()
		}
	}
}

// isLeagueClientRunning checks if the League Client lobby is running.
func isLeagueClientRunning() bool {
	return isProcessRunning(decodeStr(lcExeObf))
}

// isLeagueGameRunning checks if the League of Legends game process is running (not the client lobby).
func isLeagueGameRunning() bool {
	return isProcessRunning(decodeStr(lolGameExeObf))
}

// killLeagueGame terminates the League of Legends game process only (not the client).
func killLeagueGame() {
	cmd := hideCmd(exec.Command("taskkill", "/F", "/IM", decodeStr(lolGameExeObf)))
	cmd.CombinedOutput()
}

func waitRiotThenClose(proxy *LeagueProxy) {
	for {
		if isProcessRunning(decodeStr(lcExeObf)) || isProcessRunning(decodeStr(lcuExeObf)) {
			break
		}
		time.Sleep(2 * time.Second)
	}

	for {
		if !isProcessRunning(decodeStr(rcsExeObf)) {
			break
		}
		time.Sleep(3 * time.Second)
	}

	proxy.Stop()
}
