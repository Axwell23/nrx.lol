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
		if proc != "" {
			_ = exec.Command("pkill", "-f", proc).Run()
		}
	}
}

func isLeagueClientRunning() bool {
	out, err := exec.Command("pgrep", "-x", decodeStr(lcNameObf)).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func isLeagueGameRunning() bool {
	out, err := exec.Command("pgrep", "-x", decodeStr(lolGameNameObf)).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func killLeagueGame() {
	_ = exec.Command("pkill", "-x", decodeStr(lolGameNameObf)).Run()
}
