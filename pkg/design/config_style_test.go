package design

import "testing"

func TestGetStyleWithFallbackPrefersFirstAvailable(t *testing.T) {
	cfg := DefaultConfig()

	// Ensure predictable color availability for the test
	cfg.Colors.Warning = ""    // simulate missing first color
	cfg.Colors.Process = "99"  // primary fallback
	cfg.Colors.Success = "199" // secondary fallback

	expected := cfg.GetStyle("Process").Render("text")
	result := cfg.GetStyleWithFallback("Warning", "Process", "Success").Render("text")

	if result != expected {
		t.Fatalf("expected Process color to be used, got %q", result)
	}
}

func TestGetStyleWithFallbackMonochrome(t *testing.T) {
	cfg := DefaultConfig()
	ApplyMonochromeDefaults(cfg)

	rendered := cfg.GetStyleWithFallback("Process", "Success").Render("text")
	if rendered != "text" {
		t.Fatalf("expected unstyled text in monochrome mode, got %q", rendered)
	}
}
