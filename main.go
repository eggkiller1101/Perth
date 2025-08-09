package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"perth/player"
)

func main() {
	p := player.New()
	defer p.Close()

	fmt.Println("🎵 Perth Music Player")
	fmt.Println("Commands:")
	fmt.Println("  load <file>     - Load an audio file")
	fmt.Println("  play            - Start/Resume playback")
	fmt.Println("  pause           - Pause playback")
	fmt.Println("  stop            - Stop and reset to beginning")
	fmt.Println("  seek <seconds>  - Seek to position (in seconds)")
	fmt.Println("  volume <0-100>  - Set volume (0-100)")
	fmt.Println("  status          - Show current status")
	fmt.Println("  ls              - List available audio files")
	fmt.Println("  quit            - Exit the player")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Validate UTF-8 encoding
		if !utf8.ValidString(input) {
			fmt.Println("❌ Invalid character encoding. Please ensure your terminal supports UTF-8.")
			continue
		}

		command, args := parseCommand(input)

		switch command {
		case "load":
			if len(args) < 1 {
				fmt.Println("Usage: load <file>")
				continue
			}
			loadFile(p, args[0])

		case "play":
			if err := p.Play(); err != nil {
				fmt.Printf("Error playing: %v\n", err)
			} else {
				fmt.Println("▶️  Playing")
			}

		case "pause":
			p.Pause()
			fmt.Println("⏸️  Paused")

		case "stop":
			p.Stop()
			fmt.Println("⏹️  Stopped")

		case "seek":
			if len(args) < 1 {
				fmt.Println("Usage: seek <seconds>")
				continue
			}
			seekTo(p, args[0])

		case "volume":
			if len(args) < 1 {
				fmt.Println("Usage: volume <0-100>")
				continue
			}
			setVolume(p, args[0])

		case "status":
			showStatus(p)

		case "ls":
			listAudioFiles()

		case "quit", "exit":
			fmt.Println("👋 Goodbye!")
			return

		default:
			fmt.Printf("Unknown command: %s\n", command)
		}
	}
}

// parseCommand parses user input with proper Unicode support
func parseCommand(input string) (string, []string) {
	// Split by whitespace while preserving Unicode characters
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	command := strings.ToLower(parts[0])
	var args []string

	if len(parts) > 1 {
		switch command {
		case "load":
			// For load command, join all remaining parts to handle filenames with spaces
			// This preserves Unicode characters in filenames
			filename := strings.Join(parts[1:], " ")
			// Clean the filename and validate it exists
			filename = strings.TrimSpace(filename)
			args = []string{filename}
		case "seek", "volume":
			// These commands expect a single numeric argument
			args = []string{parts[1]}
		default:
			args = parts[1:]
		}
	}

	return command, args
}

func loadFile(p *player.Player, filePath string) {
	// Validate UTF-8 in filename
	if !utf8.ValidString(filePath) {
		fmt.Println("❌ Invalid character encoding in filename. Please ensure your terminal supports UTF-8.")
		return
	}

	// Clean and normalize the file path
	filePath = strings.TrimSpace(filePath)

	// Expand relative paths
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err == nil {
			filePath = abs
		}
	}

	// Check if file exists before attempting to load
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("❌ File not found: %s\n", filePath)
		fmt.Println("💡 Tip: Use 'ls assets/' to see available files")
		return
	}

	fmt.Printf("🎵 Loading: %s\n", filepath.Base(filePath))
	if err := p.Load(filePath); err != nil {
		fmt.Printf("❌ Error loading file: %v\n", err)
		return
	}

	duration := p.Duration()
	if duration > 0 {
		fmt.Printf("✅ Loaded successfully (Duration: %s)\n", formatDuration(duration))
	} else {
		fmt.Println("✅ Loaded successfully")
	}
}

func seekTo(p *player.Player, secondsStr string) {
	seconds, err := strconv.ParseFloat(secondsStr, 64)
	if err != nil {
		fmt.Printf("Invalid time: %s\n", secondsStr)
		return
	}

	pos := time.Duration(seconds * float64(time.Second))
	if err := p.Seek(pos); err != nil {
		fmt.Printf("Error seeking: %v\n", err)
		return
	}

	fmt.Printf("⏩ Seeked to %s\n", formatDuration(pos))
}

func setVolume(p *player.Player, volumeStr string) {
	volume, err := strconv.ParseFloat(volumeStr, 64)
	if err != nil {
		fmt.Printf("Invalid volume: %s\n", volumeStr)
		return
	}

	if volume < 0 || volume > 100 {
		fmt.Println("Volume must be between 0 and 100")
		return
	}

	// Convert percentage to linear volume (0.0 to 1.0)
	linearVolume := volume / 100.0
	p.SetVolume(linearVolume)
	fmt.Printf("🔊 Volume set to %.0f%%\n", volume)
}

func showStatus(p *player.Player) {
	position := p.Position()
	duration := p.Duration()

	fmt.Printf("📊 Status:\n")
	fmt.Printf("  Position: %s\n", formatDuration(position))
	if duration > 0 {
		fmt.Printf("  Duration: %s\n", formatDuration(duration))
		progress := float64(position) / float64(duration) * 100
		fmt.Printf("  Progress: %.1f%%\n", progress)
	}
}

func listAudioFiles() {
	fmt.Println("🎵 Available audio files:")

	// List files in assets directory
	entries, err := os.ReadDir("assets")
	if err != nil {
		fmt.Printf("❌ Cannot read assets directory: %v\n", err)
		return
	}

	audioExtensions := map[string]bool{
		".mp3":  true,
		".wav":  true,
		".flac": true,
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if audioExtensions[ext] {
			// Display the full relative path that users should type
			fmt.Printf("  assets/%s\n", entry.Name())
			found = true
		}
	}

	if !found {
		fmt.Println("  No audio files found in assets/ directory")
	} else {
		fmt.Println("\n💡 Copy and paste the full path (including 'assets/') to load a file")
	}
}

func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
