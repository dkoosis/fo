package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidkoosis/fo/internal/design"
)

func TestGetConfigPath_ReturnsLocalConfig_When_FileExists(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("FO_DEBUG", "")

	localConfig := filepath.Join(tempDir, ".fo.yaml")
	if err := os.WriteFile(localConfig, []byte("label: test\n"), 0o600); err != nil {
		t.Fatalf("failed to write local config: %v", err)
	}

	got := getConfigPath()
	if got != ".fo.yaml" {
		t.Fatalf("expected local config path, got %q", got)
	}
}

func TestGetConfigPath_UsesXDGPath_When_LocalMissing(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("FO_DEBUG", "")

	xdgRoot := filepath.Join(tempDir, "xdg")
	configHome := filepath.Join(xdgRoot, "fo")
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		t.Fatalf("failed to create XDG config directory: %v", err)
	}
	configPath := filepath.Join(configHome, ".fo.yaml")
	if err := os.WriteFile(configPath, []byte("label: xdg\n"), 0o600); err != nil {
		t.Fatalf("failed to write XDG config: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgRoot)
	t.Setenv("HOME", filepath.Join(tempDir, "home"))

	got := getConfigPath()
	if got != configPath {
		t.Fatalf("expected XDG config path %q, got %q", configPath, got)
	}
}

func TestGetConfigPath_ReturnsEmpty_When_NoConfigAvailable(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("FO_DEBUG", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "xdg"))
	t.Setenv("HOME", filepath.Join(tempDir, "home"))

	got := getConfigPath()
	if got != "" {
		t.Fatalf("expected empty config path, got %q", got)
	}
}

func TestLoadConfig_MergesYAMLOverrides_When_FilePresent(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("FO_DEBUG", "")

	yamlContent := "" +
		"label: custom\n" +
		"stream: true\n" +
		"show_output: always\n" +
		"no_timer: true\n" +
		"no_color: true\n" +
		"ci: true\n" +
		"debug: true\n" +
		"max_buffer_size: 2048\n" +
		"max_line_length: 4096\n" +
		"active_theme: ascii_minimal\n" +
		"presets:\n" +
		"  build:\n" +
		"    label: build label\n"

	if err := os.WriteFile(".fo.yaml", []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg := LoadConfig()

	if cfg.Label != "custom" || cfg.Stream != true || cfg.ShowOutput != "always" {
		t.Fatalf("unexpected core config values: %+v", cfg)
	}
	if cfg.NoTimer != true || cfg.NoColor != true || cfg.CI != true || cfg.Debug != true {
		t.Fatalf("unexpected boolean flags: %+v", cfg)
	}
	if cfg.MaxBufferSize != 2048 || cfg.MaxLineLength != 4096 {
		t.Fatalf("unexpected numeric limits: buffer=%d line=%d", cfg.MaxBufferSize, cfg.MaxLineLength)
	}
	if cfg.ActiveThemeName != "ascii_minimal" {
		t.Fatalf("expected active theme ascii_minimal, got %s", cfg.ActiveThemeName)
	}
	if cfg.Presets["build"] == nil || cfg.Presets["build"].Label != "build label" {
		t.Fatalf("preset not loaded: %+v", cfg.Presets)
	}
}

func TestLoadConfig_ReturnsDefaults_When_NoConfigFound(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("FO_DEBUG", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "xdg"))
	t.Setenv("HOME", filepath.Join(tempDir, "home"))

	cfg := LoadConfig()

	if cfg.ShowOutput != DefaultShowOutput {
		t.Fatalf("expected default show_output %s, got %s", DefaultShowOutput, cfg.ShowOutput)
	}
	if cfg.MaxBufferSize != DefaultMaxBufferSize || cfg.MaxLineLength != DefaultMaxLineLength {
		t.Fatalf("expected default limits, got buffer=%d line=%d", cfg.MaxBufferSize, cfg.MaxLineLength)
	}
	if cfg.ActiveThemeName != DefaultActiveThemeName {
		t.Fatalf("expected default theme %s, got %s", DefaultActiveThemeName, cfg.ActiveThemeName)
	}
	if cfg.Stream || cfg.NoTimer || cfg.NoColor || cfg.CI || cfg.Debug {
		t.Fatalf("expected default booleans to be false, got %+v", cfg)
	}
	if cfg.Themes[DefaultActiveThemeName] == nil {
		t.Fatalf("expected default themes to be initialized")
	}
}

func TestMergeWithFlags_AppliesEnvOverrides_When_NoColorVariablesSet(t *testing.T) {
	t.Setenv("FO_NO_COLOR", "true")
	t.Setenv("FO_CI", "")
	t.Setenv("CI", "")

	appCfg := &AppConfig{
		ActiveThemeName: DefaultActiveThemeName,
		Themes: map[string]*design.Config{
			DefaultActiveThemeName: design.UnicodeVibrantTheme(),
		},
	}

	result := MergeWithFlags(appCfg, CliFlags{})

	if result == nil {
		t.Fatalf("expected design config, got nil")
	}
	if !result.IsMonochrome {
		t.Fatalf("expected monochrome mode from env overrides")
	}
	if result.Style.UseBoxes {
		t.Fatalf("expected boxes disabled in monochrome mode")
	}
}

func TestMergeWithFlags_UsesCLITheme_When_PresentAndAvailable(t *testing.T) {
	appCfg := &AppConfig{
		ActiveThemeName: DefaultActiveThemeName,
		Themes: map[string]*design.Config{
			DefaultActiveThemeName: design.UnicodeVibrantTheme(),
			"ascii_minimal":        design.ASCIIMinimalTheme(),
		},
	}

	cliFlags := CliFlags{ThemeName: "ascii_minimal"}

	result := MergeWithFlags(appCfg, cliFlags)

	if result.ThemeName != "ascii_minimal" {
		t.Fatalf("expected theme name to follow CLI flag, got %s", result.ThemeName)
	}
	if result == appCfg.Themes["ascii_minimal"] {
		t.Fatalf("expected returned config to be a copy, not the original theme pointer")
	}
	if !result.IsMonochrome {
		t.Fatalf("expected ascii_minimal theme to retain monochrome setting")
	}
}

func TestApplyCommandPreset_UpdatesLabel_When_CommandMatches(t *testing.T) {
	appCfg := &AppConfig{
		Presets: map[string]*design.ToolConfig{
			"build":     {Label: "short"},
			"cmd/build": {Label: "full path"},
		},
	}

	ApplyCommandPreset(appCfg, "cmd/build")

	if appCfg.Label != "full path" {
		t.Fatalf("expected full path preset to apply first, got label %q", appCfg.Label)
	}
}
