package config

import (
	"os"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestResolveConfig_PriorityOrder(t *testing.T) {
	tests := []struct {
		name           string
		cliFlags       CliFlags
		envVars        map[string]string
		wantThemeSource string
		wantNoColorSource string
	}{
		{
			name: "CLI theme-file has highest priority",
			cliFlags: CliFlags{
				ThemeFile: "testdata/custom_theme.yaml",
			},
			wantThemeSource: "cli-file",
		},
		{
			name: "CLI theme-name has priority over env",
			cliFlags: CliFlags{
				ThemeName: "ascii_minimal",
			},
			envVars: map[string]string{
				"FO_THEME": "unicode_vibrant",
			},
			wantThemeSource: "cli-name",
		},
		{
			name: "CLI no-color has priority over env",
			cliFlags: CliFlags{
				NoColor:   true,
				NoColorSet: true,
			},
			envVars: map[string]string{
				"FO_NO_COLOR": "false",
			},
			wantNoColorSource: "cli",
		},
		{
			name: "Env has priority over file",
			envVars: map[string]string{
				"FO_THEME": "ascii_minimal",
			},
			wantThemeSource: "env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			resolved, err := ResolveConfig(tt.cliFlags)
			if err != nil {
				t.Fatalf("ResolveConfig() error = %v", err)
			}

			if resolved.ThemeSource != tt.wantThemeSource {
				t.Errorf("ThemeSource = %v, want %v", resolved.ThemeSource, tt.wantThemeSource)
			}

			if tt.wantNoColorSource != "" && resolved.NoColorSource != tt.wantNoColorSource {
				t.Errorf("NoColorSource = %v, want %v", resolved.NoColorSource, tt.wantNoColorSource)
			}
		})
	}
}

func TestResolveConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cliFlags CliFlags
		wantErr bool
	}{
		{
			name: "valid config",
			cliFlags: CliFlags{
				ShowOutput: "always",
			},
			wantErr: false,
		},
		{
			name: "invalid show_output",
			cliFlags: CliFlags{
				ShowOutput:    "invalid",
				ShowOutputSet: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveConfig(tt.cliFlags)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveConfig_CIModeOverrides(t *testing.T) {
	cliFlags := CliFlags{
		CI:    true,
		CISet: true,
	}

	resolved, err := ResolveConfig(cliFlags)
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}

	if !resolved.NoColor {
		t.Error("CI mode should set NoColor to true")
	}

	if !resolved.NoTimer {
		t.Error("CI mode should set NoTimer to true")
	}

	if !resolved.Theme.IsMonochrome {
		t.Error("CI mode should set theme to monochrome")
	}
}

func TestResolveTheme_Priority(t *testing.T) {
	appCfg := &AppConfig{
		ActiveThemeName: "unicode_vibrant",
		Themes:          design.DefaultThemes(),
	}

	tests := []struct {
		name           string
		cliFlags       CliFlags
		wantSource     string
		wantThemeName  string
	}{
		{
			name: "CLI theme-file",
			cliFlags: CliFlags{
				ThemeFile: "testdata/custom_theme.yaml",
			},
			wantSource: "cli-file",
		},
		{
			name: "CLI theme-name",
			cliFlags: CliFlags{
				ThemeName: "ascii_minimal",
			},
			wantSource:    "cli-name",
			wantThemeName: "ascii_minimal",
		},
		{
			name:       "file active_theme",
			cliFlags:   CliFlags{},
			wantSource: "file",
		},
		{
			name:       "default fallback",
			cliFlags:   CliFlags{},
			wantSource: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme, source := resolveTheme(tt.cliFlags, appCfg)
			if source != tt.wantSource {
				t.Errorf("resolveTheme() source = %v, want %v", source, tt.wantSource)
			}
			if theme == nil {
				t.Error("resolveTheme() returned nil theme")
			}
		})
	}
}

