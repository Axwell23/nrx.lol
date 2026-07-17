//go:build windows

package main

import (
	"os/exec"
	"strings"
)

func isServiceRunning(name string) bool {
	cmd := hideCmd(exec.Command("sc", "query", name))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "RUNNING")
}

// StopVanguard stops all Vanguard services and disables them.
func StopVanguard() {
	// Stop vgc (Vanguard user-mode service)
	hideCmd(exec.Command("sc", "stop", decodeStr(vgcServiceObf))).CombinedOutput()
	hideCmd(exec.Command("sc", "config", decodeStr(vgcServiceObf), "start=", "disabled")).CombinedOutput()

	// Stop vgk (Vanguard kernel driver)
	hideCmd(exec.Command("sc", "stop", decodeStr(vgkServiceObf))).CombinedOutput()
	hideCmd(exec.Command("sc", "config", decodeStr(vgkServiceObf), "start=", "disabled")).CombinedOutput()
}

// IsVanguardRunning checks if any Vanguard service is active.
func IsVanguardRunning() bool {
	return isServiceRunning(decodeStr(vgcServiceObf)) || isServiceRunning(decodeStr(vgkServiceObf))
}
