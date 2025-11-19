package main

import (
	"os"
	"testing"

	config "github.com/davidkoosis/fo/internal/config"
)

func TestFindCommandArgs_WithDoubleDash(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"fo", "--label", "test", "--", "echo", "hello"}
	result := findCommandArgs()
	if len(result) != 2 || result[0] != "echo" || result[1] != "hello" {
		t.Errorf("expected [echo hello], got %v", result)
	}
}

func TestFindCommandArgs_NoDoubleDash(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"fo", "--label", "test", "echo", "hello"}
	result := findCommandArgs()
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestFindCommandArgs_DoubleDashAtEnd(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"fo", "--label", "test", "--"}
	result := findCommandArgs()
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestConvertAppConfigToLocal_DefaultValues(t *testing.T) {
	appCfg := &config.AppConfig{
		Label:         "test-label",
		Stream:        true,
		ShowOutput:    "always",
		NoTimer:       true,
		NoColor:       true,
		CI:            true,
		MaxBufferSize: 5 * 1024 * 1024,
		MaxLineLength: 512 * 1024,
	}

	local := convertAppConfigToLocal(appCfg)

	if local.Label != "test-label" {
		t.Errorf("expected Label 'test-label', got '%s'", local.Label)
	}
	if !local.Stream {
		t.Error("expected Stream true")
	}
	if local.ShowOutput != "always" {
		t.Errorf("expected ShowOutput 'always', got '%s'", local.ShowOutput)
	}
	if !local.NoTimer {
		t.Error("expected NoTimer true")
	}
	if !local.NoColor {
		t.Error("expected NoColor true")
	}
	if !local.CI {
		t.Error("expected CI true")
	}
	if local.Debug {
		t.Error("expected Debug false (always defaults to false)")
	}
	if local.MaxBufferSize != 5*1024*1024 {
		t.Errorf("expected MaxBufferSize 5MB, got %d", local.MaxBufferSize)
	}
	if local.MaxLineLength != 512*1024 {
		t.Errorf("expected MaxLineLength 512KB, got %d", local.MaxLineLength)
	}
}

func TestConvertAppConfigToLocal_DebugAlwaysFalse(t *testing.T) {
	appCfg := &config.AppConfig{
		Debug: true, // Even if set to true in config
	}

	local := convertAppConfigToLocal(appCfg)

	// Debug should always be false from config, only enabled by explicit flag
	if local.Debug {
		t.Error("Debug should always be false from convertAppConfigToLocal")
	}
}

func TestLocalAppConfig_Fields(t *testing.T) {
	cfg := LocalAppConfig{
		Label:         "my-task",
		Stream:        true,
		ShowOutput:    "on-fail",
		NoTimer:       false,
		NoColor:       false,
		CI:            false,
		Debug:         true,
		MaxBufferSize: 10 * 1024 * 1024,
		MaxLineLength: 1 * 1024 * 1024,
	}

	if cfg.Label != "my-task" {
		t.Error("Label field mismatch")
	}
	if !cfg.Stream {
		t.Error("Stream field mismatch")
	}
	if cfg.ShowOutput != "on-fail" {
		t.Error("ShowOutput field mismatch")
	}
}
