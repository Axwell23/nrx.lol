//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

func terminateRiotServices() {
	processes := []string{
		decodeStr(rcsNameObf),
		decodeStr(rcuNameObf),
		decodeStr(lcNameObf),
		decodeStr(lcuNameObf),
	}
	for _, proc := range processes {
		exec.Command("pkill", "-f", proc).Run()
	}
}

func waitRiotThenClose(proxy *LeagueProxy) {
	// macOS worker doesn't wait for game processes
	proxy.Stop()
}

// isLeagueClientRunning checks if the League Client lobby is running.
func isLeagueClientRunning() bool {
	out, err := exec.Command("pgrep", "-x", decodeStr(lcNameObf)).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// isLeagueGameRunning checks if the League of Legends game process is running (not the client lobby).
func isLeagueGameRunning() bool {
	out, err := exec.Command("pgrep", "-x", decodeStr(lolGameNameObf)).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// killLeagueGame terminates the League of Legends game process only (not the client).
func killLeagueGame() {
	exec.Command("pkill", "-x", decodeStr(lolGameNameObf)).Run()
}
