package design

import (
	"testing"
)

func BenchmarkGetColor_SwitchBased(b *testing.B) {
	b.Setenv("FO_USE_REFLECTION_COLORS", "")
	cfg := UnicodeVibrantTheme()
	colors := []string{"process", "error", "warning", "success", "paleblue", "muted"}

	b.ResetTimer()
	for range b.N {
		for _, color := range colors {
			_ = cfg.GetColor(color)
		}
	}
}

func BenchmarkGetColor_ReflectionBased(b *testing.B) {
	b.Setenv("FO_USE_REFLECTION_COLORS", "1")
	cfg := UnicodeVibrantTheme()
	colors := []string{"process", "error", "warning", "success", "paleblue", "muted"}

	b.ResetTimer()
	for range b.N {
		for _, color := range colors {
			_ = cfg.GetColor(color)
		}
	}
}
