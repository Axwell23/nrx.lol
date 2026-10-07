//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func FindRiotClientServices() string {
	p := decodeStr(macRCSPathObf)
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		return p
	}
	return ""
}

func LaunchRCSLogin(_ string, _ int, _ string) error {
	return nil
}

func LaunchRCSGame(_ string, _ int, _, _ string) error {
	return nil
}

func saveYamlSettings(yamlData string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsDir := filepath.Join(homeDir, "Library", "Application Support", "Riot Games", "Riot Client", "Data")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(settingsDir, decodeStr(riotSettingsFileObf)), []byte(yamlData), 0644)
}

func launchLeague() error {
	p := decodeStr(macRCSPathObf)
	if info, err := os.Stat(p); err != nil || info.IsDir() {
		return err
	}
	cmd := exec.Command(p,
		"--launch-patchline="+decodeStr(patchlineLiveObf),
		"--launch-product="+decodeStr(productLoLObf),
	)
	return cmd.Start()
}

func clearSavedLogin() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	p := filepath.Join(homeDir, "Library", "Application Support", "Riot Games", "Riot Client", "Data", decodeStr(riotSettingsFileObf))
	_ = os.Remove(p)
}

func getSettingsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "Library", "Application Support", "Riot Games", "Riot Client", "Data", decodeStr(riotSettingsFileObf))
}

func clearRiotLogs() {}
