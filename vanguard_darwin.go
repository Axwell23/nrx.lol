//go:build darwin

package main

func StopVanguard()           {}
func IsVanguardRunning() bool { return false }
