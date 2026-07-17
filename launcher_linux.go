//go:build linux

package main

func FindRiotClientServices() string { return "" }

func LaunchRCSLogin(_ string, _ int, _ string) error { return nil }

func LaunchRCSGame(_ string, _ int, _, _ string) error { return nil }

func clearSavedLogin() {}

func getSettingsPath() string { return "" }

func saveYamlSettings(_ string) error { return nil }

func launchLeague() error { return nil }
