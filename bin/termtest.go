package main

import "fmt"

func main() {
	// --- General Info ---
	fmt.Println("--- Terminal Capability Test ---")
	fmt.Println("Note: If you see raw escape codes (like '[1m'), your terminal might not be interpreting them.")
	fmt.Println("The default macOS Terminal.app should handle these well.")
	fmt.Println()

	// --- Basic Styles ---
	fmt.Println("--- Basic Styles ---")
	fmt.Println("\033[1mThis should be BOLD.\033[0m (Normal)")
	fmt.Println("\033[4mThis should be UNDERLINED.\033[0m (Normal)")
	fmt.Println("\033[3mThis might be ITALIC (font/terminal dependent).\033[0m (Normal)")
	fmt.Println("\033[7mThis should be REVERSED (inverse video).\033[0m (Normal)")
	fmt.Println("\033[1;4;3mCombined: BOLD, UNDERLINED, ITALIC.\033[0m (Normal)")
	fmt.Println()

	// --- Standard 8/16 ANSI Colors (Foreground) ---
	fmt.Println("--- Standard 8/16 ANSI Colors (Foreground) ---")
	for i := 30; i <= 37; i++ {
		fmt.Printf("\033[%dmColor %d\033[0m  ", i, i)
	}
	fmt.Println()
	// Bright/High-Intensity versions
	for i := 90; i <= 97; i++ {
		fmt.Printf("\033[%dmColor %d\033[0m  ", i, i)
	}
	fmt.Println()
	fmt.Println()

	// --- Standard 8/16 ANSI Colors (Background) ---
	fmt.Println("--- Standard 8/16 ANSI Colors (Background) ---")
	for i := 40; i <= 47; i++ {
		fmt.Printf("\033[%d;37m Background %d \033[0m  ", i, i) // White text for visibility
	}
	fmt.Println()
	// Bright/High-Intensity versions
	for i := 100; i <= 107; i++ {
		fmt.Printf("\033[%d;30m Background %d \033[0m  ", i, i) // Black text for visibility
	}
	fmt.Println()
	fmt.Println()

	// --- 256 Colors (Sample) ---
	fmt.Println("--- 256 Colors (Sample Palette - Foreground) ---")
	fmt.Println("If your terminal supports 256 colors (e.g., TERM=xterm-256color):")
	// Sample some colors from the 6x6x6 cube (16-231)
	for i := 0; i < 6; i++ {
		colorCode := 16 + i*36 // Start of each red block
		fmt.Printf("\033[38;5;%dmColor %3d\033[0m  ", colorCode, colorCode)
	}
	fmt.Println()
	// Sample some grayscale colors (232-255)
	for i := 0; i < 6; i++ {
		colorCode := 232 + i*4
		fmt.Printf("\033[38;5;%dmColor %3d\033[0m  ", colorCode, colorCode)
	}
	fmt.Println()
	fmt.Println()

	// --- True Color (24-bit) Test (Sample) ---
	fmt.Println("--- True Color (24-bit RGB - Sample) ---")
	fmt.Println("If your terminal supports true color:")
	fmt.Println("\033[38;2;255;100;100mThis should be a light red (RGB 255,100,100).\033[0m")
	fmt.Println("\033[48;2;100;255;100;30mThis should be black text on light green background (RGB 100,255,100).\033[0m")
	fmt.Println()

	// --- Unicode Characters ---
	fmt.Println("--- Unicode Characters ---")
	fmt.Println("Box Drawing: ┌─┬─┐ │ │ │ ├─┼─┤ └─┴─┘ (Light)")
	fmt.Println("Box Drawing: ┏━┳━┓ ┃ ┃ ┃ ┣━╋━┫ ┗━┻━┛ (Heavy)")
	fmt.Println("Block Elements: █ ▓ ▒ ░")
	fmt.Println("Symbols: ✔ ✅ ✘ ❌ ℹ️ ⚠️ ▶ ➤ ● ★ ☆ … ≠ ≤ ≥")
	fmt.Println("Test for wide characters (e.g., Japanese): こんにちは世界 (Hello World)")
	fmt.Println()

	// --- Cursor Movement (Basic - might not be easily verifiable without more complex logic) ---
	// This just prints the codes; visual verification is harder.
	// A true test would involve reading cursor position, which is more advanced.
	fmt.Println("--- Cursor Control (Sending Codes - Visual effect varies) ---")
	fmt.Print("Line 1")
	fmt.Print("\033[5D")  // Move cursor left 5 columns
	fmt.Print("Overlap?") // Should overlap "Line " if supported and font is monospaced
	fmt.Println()
	fmt.Print("Line 2\033[1A\033[5CUP_AND_RIGHT\n") // Move up 1, right 5, then text, then newline to restore
	fmt.Println()

	fmt.Println("--- Test Complete ---")
}
