package design

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColor_SprintsWithReset_When_CodeIsProvided(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		color    Color
		input    string
		expected string
	}{
		{
			name:     "appends reset when color code is present",
			color:    NewColor("\u001b[31m"),
			input:    "error",
			expected: "\u001b[31merror" + ANSIReset,
		},
		{
			name:     "returns input unchanged when code empty",
			color:    NewColor(""),
			input:    "plain",
			expected: "plain",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			styled := tc.color.Sprint(tc.input)

			assert.Equal(t, tc.expected, styled)
			assert.Equal(t, tc.color.code, tc.color.Code())
			assert.Equal(t, tc.color.code == "", tc.color.IsEmpty())
		})
	}
}

func TestNormalizeANSIEscape_ReturnsNormalizedSequence_When_ProvidedVariousInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "keeps existing escape prefix",
			input:    "\x1b[31m",
			expected: "\x1b[31m",
		},
		{
			name:     "converts literal hex sequence",
			input:    "\\x1b[32m",
			expected: "\x1b[32m",
		},
		{
			name:     "returns empty when input empty",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, NormalizeANSIEscape(tc.input))
		})
	}
}

func TestConfig_ResetsToDefault_When_ResetColorMissing(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Colors.Reset = ""

	assert.Equal(t, "\u001b[0m", cfg.ResetColor())

	cfg.IsMonochrome = true
	assert.Equal(t, "", cfg.ResetColor())
}

func TestConfig_ResolvesColors_When_UsingElementOverrides(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Elements["Task_Progress_Line"] = ElementStyleDef{ColorFG: "bluefg"}

	tests := []struct {
		name        string
		color       string
		elementName string
		expected    string
	}{
		{
			name:        "uses element style override",
			color:       "process",
			elementName: "Task_Progress_Line",
			expected:    "\u001b[0;34m",
		},
		{
			name:     "passes through raw escape",
			color:    "\u001b[35m",
			expected: "\u001b[35m",
		},
		{
			name:     "falls back to reset for unknown name",
			color:    "unknown-key",
			expected: "\u001b[0m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := cfg.GetColor(tc.color, tc.elementName)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestConfig_ReturnsColorWrapper_When_ColorKeyExists(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()

	t.Run("returns ANSI wrapped color", func(t *testing.T) {
		t.Parallel()

		color := cfg.GetColorObj(ColorKeyError)
		require.False(t, color.IsEmpty())
		assert.Contains(t, color.Sprint("boom"), cfg.Colors.Error)
	})

	t.Run("falls back to default escape when color missing", func(t *testing.T) {
		t.Parallel()

		cfgCopy := DeepCopyConfig(cfg)
		cfgCopy.Colors.Error = ""

		color := cfgCopy.GetColorObj(ColorKeyError)
		// Missing color should use the default ANSI code for errors.
		assert.Contains(t, color.Sprint("boom"), "[0;31mboom")
	})
}

func TestConfig_UsesAsciiDefaults_When_NoColorPresetRequested(t *testing.T) {
	t.Parallel()

	cfg := NoColorConfig()

	t.Run("configures monochrome ascii theme", func(t *testing.T) {
		t.Parallel()

		require.NotNil(t, cfg)
		assert.True(t, cfg.IsMonochrome)
		assert.Equal(t, "no_color_derived_from_ascii", cfg.ThemeName)
		assert.Equal(t, BorderNone, cfg.Border.TaskStyle)
		assert.Empty(t, cfg.Colors.Process)
		assert.Equal(t, IconBullet, cfg.Icons.Bullet)
	})
}

func TestDefaultConfig_ReturnsVibrantTheme_When_NoOverridesProvided(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	require.NotNil(t, cfg)
	assert.False(t, cfg.IsMonochrome)
	assert.Equal(t, "unicode_vibrant", cfg.ThemeName)
	assert.Equal(t, BorderLeftDouble, cfg.Border.TaskStyle)
}

func TestDeepCopyConfig_CreatesIndependentCopy_When_ConfigContainsNestedStructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source *Config
	}{
		{
			name:   "creates independent copy with nested state",
			source: UnicodeVibrantTheme(),
		},
		{
			name:   "returns nil when source is nil",
			source: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.source != nil {
				tc.source.Patterns.Intent["deploy"] = []string{"blue"}
				tc.source.Elements["Custom"] = ElementStyleDef{Text: "original"}
			}

			cfgCopy := DeepCopyConfig(tc.source)

			if tc.source == nil {
				assert.Nil(t, cfgCopy)
				return
			}

			require.NotNil(t, cfgCopy)

			// Mutate copy to ensure original is unaffected.
			cfgCopy.Patterns.Intent["deploy"][0] = "green"
			cfgCopy.Elements["Custom"] = ElementStyleDef{Text: "modified"}

			assert.Equal(t, []string{"blue"}, tc.source.Patterns.Intent["deploy"])
			assert.Equal(t, ElementStyleDef{Text: "original"}, tc.source.Elements["Custom"])
		})
	}
}

func TestApplyMonochromeDefaults_ClearsColorStyling_When_ConfigHasExistingColors(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Elements["Task_Label_Header"] = ElementStyleDef{ColorFG: "Process", ColorBG: "Error", Text: "HEADER"}

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "updates existing config to monochrome",
			cfg:  cfg,
		},
		{
			name: "returns safely when config is nil",
			cfg:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ApplyMonochromeDefaults(tc.cfg)

			if tc.cfg == nil {
				return
			}

			assert.True(t, tc.cfg.IsMonochrome)
			assert.False(t, tc.cfg.Style.UseBoxes)
			assert.Empty(t, tc.cfg.Colors.Process)
			assert.Equal(t, IconStart, tc.cfg.Icons.Start)

			updated := tc.cfg.Elements["Task_Label_Header"]
			assert.Empty(t, updated.ColorFG)
			assert.Empty(t, updated.ColorBG)
			assert.Equal(t, "HEADER", updated.Text)
		})
	}
}

func TestConfig_UsesReflection_When_FeatureFlagEnabled(t *testing.T) {
	t.Parallel()

	// Save original env value
	originalEnv := os.Getenv("FO_USE_REFLECTION_COLORS")
	defer func() {
		if originalEnv != "" {
			os.Setenv("FO_USE_REFLECTION_COLORS", originalEnv)
		} else {
			os.Unsetenv("FO_USE_REFLECTION_COLORS")
		}
	}()

	tests := []struct {
		name           string
		enableReflection bool
		colorName      string
		expectedPrefix string
	}{
		{
			name:           "uses reflection when flag enabled",
			enableReflection: true,
			colorName:      "error",
			expectedPrefix: "\u001b[0;31m",
		},
		{
			name:           "uses switch when flag disabled",
			enableReflection: false,
			colorName:      "error",
			expectedPrefix: "\u001b[0;31m",
		},
		{
			name:           "reflection handles paleblue",
			enableReflection: true,
			colorName:      "paleblue",
			expectedPrefix: "\u001b[38;5;111m",
		},
		{
			name:           "switch handles paleblue",
			enableReflection: false,
			colorName:      "paleblue",
			expectedPrefix: "\u001b[38;5;111m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Set environment variable
			if tc.enableReflection {
				os.Setenv("FO_USE_REFLECTION_COLORS", "1")
			} else {
				os.Unsetenv("FO_USE_REFLECTION_COLORS")
			}

			// Create fresh config to pick up env change
			cfg := UnicodeVibrantTheme()
			result := cfg.GetColor(tc.colorName)

			// Both methods should produce same result
			assert.Contains(t, result, tc.expectedPrefix)
		})
	}
}

func TestConfig_ReflectionMatchesSwitch_When_AllColors(t *testing.T) {
	t.Parallel()

	// Save original env value
	originalEnv := os.Getenv("FO_USE_REFLECTION_COLORS")
	defer func() {
		if originalEnv != "" {
			os.Setenv("FO_USE_REFLECTION_COLORS", originalEnv)
		} else {
			os.Unsetenv("FO_USE_REFLECTION_COLORS")
		}
	}()

	cfg := UnicodeVibrantTheme()
	colorsToTest := []string{
		"process", "success", "warning", "error",
		"detail", "muted", "reset", "white",
		"greenfg", "bluefg", "bluebg", "paleblue",
		"bold", "italic",
	}

	// Test with switch-based (default)
	os.Unsetenv("FO_USE_REFLECTION_COLORS")
	switchResults := make(map[string]string)
	for _, color := range colorsToTest {
		switchResults[color] = cfg.GetColor(color)
	}

	// Test with reflection-based
	os.Setenv("FO_USE_REFLECTION_COLORS", "1")
	cfgReflection := UnicodeVibrantTheme()
	reflectionResults := make(map[string]string)
	for _, color := range colorsToTest {
		reflectionResults[color] = cfgReflection.GetColor(color)
	}

	// Compare results - they should match
	for _, color := range colorsToTest {
		t.Run(color, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, switchResults[color], reflectionResults[color],
				"Reflection and switch should produce same result for color: %s", color)
		})
	}
}
