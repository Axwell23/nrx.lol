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
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	programData := os.Getenv("ProgramData")
	if programData != "" {
		if data, err := os.ReadFile(filepath.Join(programData, "Riot Games", decodeStr(rcsInstallsJsonObf))); err == nil {
			content := string(data)
			if idx := strings.Index(content, `"`+decodeStr(rcLiveKeyObf)+`"`); idx != -1 {
				rest := content[idx+len(`"`+decodeStr(rcLiveKeyObf)+`"`):]
				if colonIdx := strings.Index(rest, `"`); colonIdx != -1 {
					rest = rest[colonIdx+1:]
					if endIdx := strings.Index(rest, `"`); endIdx != -1 {
						path := rest[:endIdx]
						path = strings.ReplaceAll(path, `\\`, `\`)
						path = strings.ReplaceAll(path, `/`, `\`)
						if _, err := os.Stat(path); err == nil {
							return path
						}
					}
				}
			}
		}
	}

	return ""
}

func LaunchRCSLogin(rcsPath string, proxyPort int, patchline string) error {
	clientConfigURL := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)
	args := []string{
		"--launch-patchline=" + patchline,
		"--client-config-url=" + clientConfigURL,
		"--disable-features=" + decodeStr(disableFeatObf),
	}
	return launchRCSWithArgs(rcsPath, proxyPort, args)
}

func LaunchRCSGame(rcsPath string, proxyPort int, product, patchline string) error {
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

	filteredEnv := make([]string, 0, len(env))
	for _, e := range env {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "HTTP_PROXY=") ||
			strings.HasPrefix(upper, "HTTPS_PROXY=") ||
			strings.HasPrefix(upper, "NO_PROXY=") {
			continue
		}
		filteredEnv = append(filteredEnv, e)
	}

	filteredEnv = append(filteredEnv,
		"HTTP_PROXY="+proxyAddr,
		"HTTPS_PROXY="+proxyAddr,
	)

	hasSystemRoot := false
	for _, e := range filteredEnv {
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
		filteredEnv = append(filteredEnv, "SYSTEMROOT="+systemRoot)
	}

	cmd.Env = filteredEnv
	return cmd.Start()
}

func clearSavedLogin() {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return
	}
	os.Remove(filepath.Join(localAppData, "Riot Games", "Riot Client", "Data", decodeStr(riotSettingsFileObf)))
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
		os.RemoveAll(filepath.Join(localAppData, "Riot Games", "Riot Client", "Logs"))
		os.RemoveAll(filepath.Join(localAppData, "Riot Games", "Riot Client", "Crashes"))
		os.RemoveAll(filepath.Join(localAppData, "Riot Games", "League of Legends", "Logs"))
	}
	for _, drive := range []string{"C", "D", "E", "F"} {
		root := drive + `:\`
		if _, err := os.Stat(root); err != nil {
			continue
		}
		os.RemoveAll(filepath.Join(root, "Riot Games", "Riot Client", "Logs"))
		os.RemoveAll(filepath.Join(root, "Riot Games", "League of Legends", "Logs"))
		os.RemoveAll(filepath.Join(root, "Riot Games", "League of Legends", "Game", "Logs"))
	}
}
