//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
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
	if name == "" {
		return false
	}

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
			_ = cmd.Run()
		}
	}
}

func isLeagueClientRunning() bool {
	return isProcessRunning(decodeStr(lcExeObf))
}

func isLeagueGameRunning() bool {
	return isProcessRunning(decodeStr(lolGameExeObf))
}

func killLeagueGame() {
	cmd := hideCmd(exec.Command("taskkill", "/F", "/IM", decodeStr(lolGameExeObf)))
	_ = cmd.Run()
}
