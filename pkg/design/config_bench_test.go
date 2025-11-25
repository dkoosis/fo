package design

import (
	"os"
	"testing"
)

func BenchmarkGetColor_SwitchBased(b *testing.B) {
	os.Unsetenv("FO_USE_REFLECTION_COLORS")
	cfg := UnicodeVibrantTheme()
	colors := []string{"process", "error", "warning", "success", "paleblue", "muted"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, color := range colors {
			_ = cfg.GetColor(color)
		}
	}
}

func BenchmarkGetColor_ReflectionBased(b *testing.B) {
	os.Setenv("FO_USE_REFLECTION_COLORS", "1")
	cfg := UnicodeVibrantTheme()
	colors := []string{"process", "error", "warning", "success", "paleblue", "muted"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, color := range colors {
			_ = cfg.GetColor(color)
		}
	}
}

