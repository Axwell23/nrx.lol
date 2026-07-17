//go:build linux

package main

func terminateRiotServices() {}

func isLeagueClientRunning() bool { return false }

func isLeagueGameRunning() bool { return false }

func killLeagueGame() {}

func waitRiotThenClose(proxy *LeagueProxy) {
	proxy.Stop()
}

func clearRiotLogs() {}
