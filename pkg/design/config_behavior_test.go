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

func TestConfig_ResetsToDefault_When_ResetColorMissing(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Colors.Reset = ""

	// ResetColor returns empty when reset color is not configured (lipgloss handles reset internally)
	assert.Equal(t, "", string(cfg.ResetColor()))

	cfg.IsMonochrome = true
	assert.Equal(t, "", string(cfg.ResetColor()))
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
			expected:    "39", // BlueFg - lipgloss color value
		},
		{
			name:     "passes through color value",
			color:    "120", // Direct lipgloss color value
			expected: "120",
		},
		{
			name:     "returns input for unknown name",
			color:    "unknown-key",
			expected: "unknown-key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := cfg.GetColor(tc.color, tc.elementName)
			assert.Equal(t, tc.expected, string(got))
		})
	}
}

func TestConfig_ReturnsLipglossColor_When_ColorKeyExists(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()

	t.Run("returns lipgloss color value", func(t *testing.T) {
		t.Parallel()

		color := cfg.GetColor(ColorKeyError)
		assert.Equal(t, "196", string(color)) // Red in 256-color palette
	})

	t.Run("falls back to default when color missing", func(t *testing.T) {
		t.Parallel()

		cfgCopy := DeepCopyConfig(cfg)
		cfgCopy.Colors.Error = ""

		color := cfgCopy.GetColor(ColorKeyError)
		// Missing color should use the default lipgloss color for errors
		assert.Equal(t, "196", string(color))
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

			// Make a copy of the source to avoid race conditions when running in parallel
			var sourceCopy *Config
			if tc.source != nil {
				sourceCopy = DeepCopyConfig(tc.source)
				sourceCopy.Patterns.Intent["deploy"] = []string{"blue"}
				sourceCopy.Elements["Custom"] = ElementStyleDef{Text: "original"}
			}

			cfgCopy := DeepCopyConfig(sourceCopy)

			if sourceCopy == nil {
				assert.Nil(t, cfgCopy)
				return
			}

			require.NotNil(t, cfgCopy)

			// Mutate copy to ensure original is unaffected.
			cfgCopy.Patterns.Intent["deploy"][0] = "green"
			cfgCopy.Elements["Custom"] = ElementStyleDef{Text: "modified"}

			assert.Equal(t, []string{"blue"}, sourceCopy.Patterns.Intent["deploy"])
			assert.Equal(t, ElementStyleDef{Text: "original"}, sourceCopy.Elements["Custom"])
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
	// Save original env value
	originalEnv := os.Getenv("FO_USE_REFLECTION_COLORS")
	t.Cleanup(func() {
		if originalEnv != "" {
			_ = os.Setenv("FO_USE_REFLECTION_COLORS", originalEnv)
		} else {
			_ = os.Unsetenv("FO_USE_REFLECTION_COLORS")
		}
	})

	tests := []struct {
		name             string
		enableReflection bool
		colorName        string
		expected         string
	}{
		{
			name:             "uses reflection when flag enabled",
			enableReflection: true,
			colorName:        "error",
			expected:         "196", // Red in 256-color palette
		},
		{
			name:             "uses switch when flag disabled",
			enableReflection: false,
			colorName:        "error",
			expected:         "196",
		},
		{
			name:             "reflection handles paleblue",
			enableReflection: true,
			colorName:        "paleblue",
			expected:         "111", // Pale blue
		},
		{
			name:             "switch handles paleblue",
			enableReflection: false,
			colorName:        "paleblue",
			expected:         "111",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable
			if tc.enableReflection {
				t.Setenv("FO_USE_REFLECTION_COLORS", "1")
			} else {
				t.Setenv("FO_USE_REFLECTION_COLORS", "")
			}

			// Create fresh config to pick up env change
			cfg := UnicodeVibrantTheme()
			result := string(cfg.GetColor(tc.colorName))

			// Both methods should produce same lipgloss color value
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConfig_ReflectionMatchesSwitch_When_AllColors(t *testing.T) {
	// Cannot use t.Parallel() because this test modifies the global environment variable
	// FO_USE_REFLECTION_COLORS, which causes race conditions when multiple tests run concurrently.

	// Save original env value
	originalEnv := os.Getenv("FO_USE_REFLECTION_COLORS")
	t.Cleanup(func() {
		if originalEnv != "" {
			_ = os.Setenv("FO_USE_REFLECTION_COLORS", originalEnv)
		} else {
			_ = os.Unsetenv("FO_USE_REFLECTION_COLORS")
		}
	})

	colorsToTest := []string{
		"process", "success", "warning", "error",
		"detail", "muted", "reset", "white",
		"greenfg", "bluefg", "bluebg", "paleblue",
		"bold", "italic",
	}

	// Test with switch-based (default)
	_ = os.Unsetenv("FO_USE_REFLECTION_COLORS") //nolint:errcheck,usetesting
	cfg := UnicodeVibrantTheme()
	switchResults := make(map[string]string)
	for _, color := range colorsToTest {
		switchResults[color] = string(cfg.GetColor(color))
	}

	// Test with reflection-based
	_ = os.Setenv("FO_USE_REFLECTION_COLORS", "1") //nolint:errcheck,usetesting
	cfgReflection := UnicodeVibrantTheme()
	reflectionResults := make(map[string]string)
	for _, color := range colorsToTest {
		reflectionResults[color] = string(cfgReflection.GetColor(color))
	}

	// Compare results - they should match
	for _, color := range colorsToTest {
		t.Run(color, func(t *testing.T) {
			// Subtests can run in parallel since they only read the pre-computed results
			t.Parallel()
			assert.Equal(t, switchResults[color], reflectionResults[color],
				"Reflection and switch should produce same result for color: %s", color)
		})
	}
}
