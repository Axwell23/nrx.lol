//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func FindRiotClientServices() string {
	searchPaths := []string{
		`E:\Riot Games\Riot Client\` + decodeStr(rcsExeObf),
		`F:\Riot Games\Riot Client\` + decodeStr(rcsExeObf),
		`C:\Riot Games\Riot Client\` + decodeStr(rcsExeObf),
		`C:\Program Files\Riot Games\Riot Client\` + decodeStr(rcsExeObf),
		`C:\Program Files (x86)\Riot Games\Riot Client\` + decodeStr(rcsExeObf),
		`D:\Riot Games\Riot Client\` + decodeStr(rcsExeObf),
	}

	for _, p := range searchPaths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}

	programData := os.Getenv("ProgramData")
	if programData != "" {
		p := filepath.Join(programData, "Riot Games", "Riot Client", decodeStr(rcsExeObf))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}

	return ""
}

func LaunchRCSLogin(rcsPath string, proxyPort int, patchline string) error {
	if rcsPath == "" {
		return fmt.Errorf("empty RCS path")
	}
	if info, err := os.Stat(rcsPath); err != nil || info.IsDir() {
		return fmt.Errorf("invalid RCS path: %w", err)
	}

	clientConfigURL := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)
	args := []string{
		"--launch-patchline=" + patchline,
		"--client-config-url=" + clientConfigURL,
		"--disable-features=" + decodeStr(disableFeatObf),
	}
	return launchRCSWithArgs(rcsPath, proxyPort, args)
}

func LaunchRCSGame(rcsPath string, proxyPort int, product, patchline string) error {
	if rcsPath == "" {
		return fmt.Errorf("empty RCS path")
	}
	if info, err := os.Stat(rcsPath); err != nil || info.IsDir() {
		return fmt.Errorf("invalid RCS path: %w", err)
	}

	clientConfigURL := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)
	args := []string{
		"--launch-patchline=" + patchline,
		"--launch-product=" + product,
		"--client-config-url=" + clientConfigURL,
		"--disable-features=" + decodeStr(disableFeatObf),
	}
	return launchRCSWithArgs(rcsPath, proxyPort, args)
}

func launchRCSWithArgs(rcsPath string, proxyPort int, args []string) error {
	cmd := exec.Command(rcsPath, args...)

	proxyAddr := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)
	env := os.Environ()

	fileredEnv := make([]string, 0, len(env)+3)
	for _, e := range env {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "HTTP_PROXY=") ||
			strings.HasPrefix(upper, "HTTPS_PROXY=") ||
			strings.HasPrefix(upper, "NO_PROXY=") ||
			strings.HasPrefix(upper, "ALL_PROXY=") ||
			strings.HasPrefix(upper, "FTP_PROXY=") {
			continue
		}
		fileredEnv = append(fileredEnv, e)
	}

	fileredEnv = append(fileredEnv,
		"HTTP_PROXY="+proxyAddr,
		"HTTPS_PROXY="+proxyAddr,
	)

	hasSystemRoot := false
	for _, e := range fileredEnv {
		if strings.HasPrefix(strings.ToUpper(e), "SYSTEMROOT=") {
			hasSystemRoot = true
			break
		}
	}
	if !hasSystemRoot {
		systemRoot := os.Getenv("SYSTEMROOT")
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		fileredEnv = append(fileredEnv, "SYSTEMROOT="+systemRoot)
	}

	cmd.Env = fileredEnv
	return cmd.Start()
}

func clearSavedLogin() {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return
	}
	_ = os.Remove(filepath.Join(localAppData, "Riot Games", "Riot Client", "Data", decodeStr(riotSettingsFileObf)))
}

func getSettingsPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, "Riot Games", "Riot Client", "Data", decodeStr(riotSettingsFileObf))
}

func saveYamlSettings(yamlData string) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return fmt.Errorf("LOCALAPPDATA not set")
	}
	settingsDir := filepath.Join(localAppData, "Riot Games", "Riot Client", "Data")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(settingsDir, decodeStr(riotSettingsFileObf)), []byte(yamlData), 0644)
}

func launchLeague() error {
	rcsPath := FindRiotClientServices()
	if rcsPath == "" {
		return fmt.Errorf("RCS not found")
	}
	cmd := exec.Command(rcsPath,
		"--launch-patchline="+decodeStr(patchlineLiveObf),
		"--launch-product="+decodeStr(productLoLObf),
	)
	return cmd.Start()
}

func clearRiotLogs() {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		_ = os.RemoveAll(filepath.Join(localAppData, "Riot Games", "Riot Client", "Logs"))
		_ = os.RemoveAll(filepath.Join(localAppData, "Riot Games", "Riot Client", "Crashes"))
		_ = os.RemoveAll(filepath.Join(localAppData, "Riot Games", "League of Legends", "Logs"))
	}
	for _, drive := range []string{"C", "D", "E", "F"} {
		root := drive + `:\`
		if _, err := os.Stat(root); err != nil {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, "Riot Games", "Riot Client", "Logs"))
		_ = os.RemoveAll(filepath.Join(root, "Riot Games", "League of Legends", "Logs"))
		_ = os.RemoveAll(filepath.Join(root, "Riot Games", "League of Legends", "Game", "Logs"))
	}
}
