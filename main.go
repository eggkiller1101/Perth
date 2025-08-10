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
	"perth/playlist"
)

func main() {
	p := player.New()
	defer p.Close()

	// Initialize playlist scanner
	playlistScanner := playlist.NewScanner([]string{"assets"})

	// Perform initial scan
	fmt.Println("🎵 Perth Music Player")
	fmt.Println("📁 Scanning for audio files...")
	result, err := playlistScanner.Scan()
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to scan audio files: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d audio tracks\n", result.TotalFiles)
	}
	fmt.Println()

	fmt.Println("Commands:")
	fmt.Println("  load <file>     - Load an audio file")
	fmt.Println("  play            - Start/Resume playback")
	fmt.Println("  pause           - Pause playback")
	fmt.Println("  stop            - Stop and reset to beginning")
	fmt.Println("  seek <seconds>  - Seek to position (in seconds)")
	fmt.Println("  volume <0-100>  - Set volume (0-100)")
	fmt.Println("  status          - Show current status")
	fmt.Println("  ls              - List available audio files")
	fmt.Println("  list            - Show playlist tracks")
	fmt.Println("  next            - Play next track in playlist")
	fmt.Println("  prev            - Play previous track in playlist")
	fmt.Println("  goto <index>    - Jump to track by index")
	fmt.Println("  rescan          - Rescan audio files")
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

		case "list":
			listPlaylistTracks(playlistScanner)

		case "next":
			playNextTrack(p, playlistScanner)

		case "prev":
			playPreviousTrack(p, playlistScanner)

		case "goto":
			if len(args) < 1 {
				fmt.Println("Usage: goto <index>")
				continue
			}
			gotoTrack(p, playlistScanner, args[0])

		case "rescan":
			rescanAudioFiles(playlistScanner)

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

// Playlist management functions

var currentTrackIndex = -1

func listPlaylistTracks(scanner *playlist.Scanner) {
	tracks := scanner.GetTracks()
	if len(tracks) == 0 {
		fmt.Println("📭 No tracks found in playlist")
		return
	}

	fmt.Printf("🎵 Playlist (%d tracks):\n", len(tracks))
	for i, track := range tracks {
		marker := "  "
		if i == currentTrackIndex {
			marker = "▶️ "
		}
		fmt.Printf("%s%d. %s\n", marker, i+1, track.String())

		// Show metadata for current track
		if i == currentTrackIndex && track.HasMetadata() {
			artist := track.Artist()
			album := track.Album()
			if artist != "" {
				fmt.Printf("     Artist: %s\n", artist)
			}
			if album != "" {
				fmt.Printf("     Album: %s\n", album)
			}
		}
	}

	if currentTrackIndex >= 0 {
		fmt.Printf("\n💡 Current track: %d\n", currentTrackIndex+1)
	}
}

func playNextTrack(p *player.Player, scanner *playlist.Scanner) {
	tracks := scanner.GetTracks()
	if len(tracks) == 0 {
		fmt.Println("📭 No tracks in playlist")
		return
	}

	if currentTrackIndex < 0 {
		currentTrackIndex = 0
	} else {
		currentTrackIndex = (currentTrackIndex + 1) % len(tracks)
	}

	track := tracks[currentTrackIndex]
	fmt.Printf("⏭️  Next track: %s\n", track.String())

	loadFile(p, track.Path)

	if err := p.Play(); err != nil {
		fmt.Printf("❌ Failed to play track: %v\n", err)
	} else {
		fmt.Println("▶️  Playing next track")
	}
}

func playPreviousTrack(p *player.Player, scanner *playlist.Scanner) {
	tracks := scanner.GetTracks()
	if len(tracks) == 0 {
		fmt.Println("📭 No tracks in playlist")
		return
	}

	if currentTrackIndex < 0 {
		currentTrackIndex = 0
	} else {
		currentTrackIndex = (currentTrackIndex - 1 + len(tracks)) % len(tracks)
	}

	track := tracks[currentTrackIndex]
	fmt.Printf("⏮️  Previous track: %s\n", track.String())

	loadFile(p, track.Path)

	if err := p.Play(); err != nil {
		fmt.Printf("❌ Failed to play track: %v\n", err)
	} else {
		fmt.Println("▶️  Playing previous track")
	}
}

func gotoTrack(p *player.Player, scanner *playlist.Scanner, indexStr string) {
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		fmt.Printf("❌ Invalid index: %s\n", indexStr)
		return
	}

	tracks := scanner.GetTracks()
	if index < 1 || index > len(tracks) {
		fmt.Printf("❌ Index out of range (1-%d)\n", len(tracks))
		return
	}

	currentTrackIndex = index - 1
	track := tracks[currentTrackIndex]
	fmt.Printf("🎯 Jumping to track %d: %s\n", index, track.String())

	loadFile(p, track.Path)

	if err := p.Play(); err != nil {
		fmt.Printf("❌ Failed to play track: %v\n", err)
	} else {
		fmt.Println("▶️  Playing selected track")
	}
}

func rescanAudioFiles(scanner *playlist.Scanner) {
	fmt.Println("🔄 Rescanning audio files...")
	result, err := scanner.Scan()
	if err != nil {
		fmt.Printf("❌ Rescan failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Rescan completed!\n")
	fmt.Printf("📊 Results:\n")
	fmt.Printf("  Total tracks: %d\n", result.TotalFiles)
	fmt.Printf("  New tracks: %d\n", result.NewTracks)
	fmt.Printf("  Updated tracks: %d\n", result.UpdatedTracks)
	fmt.Printf("  Removed tracks: %d\n", result.RemovedTracks)

	if len(result.Errors) > 0 {
		fmt.Printf("⚠️  Errors encountered:\n")
		for _, err := range result.Errors {
			fmt.Printf("    - %s\n", err)
		}
	}

	// Reset current track index if it's out of bounds
	tracks := scanner.GetTracks()
	if currentTrackIndex >= len(tracks) {
		currentTrackIndex = -1
	}
}
