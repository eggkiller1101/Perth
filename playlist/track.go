package playlist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
)

// Track represents a single audio track in the playlist
type Track struct {
	ID       string        `json:"id"`       // Unique identifier (hash of path)
	Path     string        `json:"path"`     // Absolute file path
	Filename string        `json:"filename"` // Display name (fallback)
	Duration time.Duration `json:"duration"` // Track duration
	Format   string        `json:"format"`   // Audio format (mp3, wav, flac)
	Size     int64         `json:"size"`     // File size in bytes
	Modified time.Time     `json:"modified"` // Last modification time

	// Lazy-loaded metadata
	metadata   *Metadata    `json:"-"` // Pointer to avoid copying
	metadataMu sync.RWMutex `json:"-"` // Thread-safe lazy loading
}

// Metadata contains track metadata extracted from audio files
type Metadata struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`
	Loaded      bool   `json:"loaded"`
}

// NewTrack creates a new Track instance
func NewTrack(path string, duration time.Duration, size int64, modified time.Time) *Track {
	// Generate ID from path hash for uniqueness
	id := generateID(path)

	return &Track{
		ID:       id,
		Path:     path,
		Filename: filepath.Base(path),
		Duration: duration,
		Format:   strings.ToLower(filepath.Ext(path)),
		Size:     size,
		Modified: modified,
		metadata: &Metadata{Loaded: false},
	}
}

// DisplayName returns the best available display name for the track
func (t *Track) DisplayName() string {
	t.metadataMu.RLock()
	defer t.metadataMu.RUnlock()

	if t.metadata != nil && t.metadata.Loaded && t.metadata.Title != "" {
		return t.metadata.Title
	}
	return t.Filename
}

// Artist returns the track artist (lazy-loaded)
func (t *Track) Artist() string {
	t.loadMetadata()
	t.metadataMu.RLock()
	defer t.metadataMu.RUnlock()
	if t.metadata != nil {
		return t.metadata.Artist
	}
	return ""
}

// Album returns the track album (lazy-loaded)
func (t *Track) Album() string {
	t.loadMetadata()
	t.metadataMu.RLock()
	defer t.metadataMu.RUnlock()
	if t.metadata != nil {
		return t.metadata.Album
	}
	return ""
}

// Genre returns the track genre (lazy-loaded)
func (t *Track) Genre() string {
	t.loadMetadata()
	t.metadataMu.RLock()
	defer t.metadataMu.RUnlock()
	if t.metadata != nil {
		return t.metadata.Genre
	}
	return ""
}

// Year returns the track year (lazy-loaded)
func (t *Track) Year() int {
	t.loadMetadata()
	t.metadataMu.RLock()
	defer t.metadataMu.RUnlock()
	if t.metadata != nil {
		return t.metadata.Year
	}
	return 0
}

// HasMetadata returns true if the track has loaded metadata
func (t *Track) HasMetadata() bool {
	t.metadataMu.RLock()
	defer t.metadataMu.RUnlock()
	return t.metadata != nil && t.metadata.Loaded
}

// loadMetadata loads metadata from the audio file if not already loaded
func (t *Track) loadMetadata() {
	t.metadataMu.RLock()
	if t.metadata != nil && t.metadata.Loaded {
		t.metadataMu.RUnlock()
		return
	}
	t.metadataMu.RUnlock()

	t.metadataMu.Lock()
	defer t.metadataMu.Unlock()

	// Double-check after acquiring write lock
	if t.metadata != nil && t.metadata.Loaded {
		return
	}

	// Ensure metadata is initialized
	if t.metadata == nil {
		t.metadata = &Metadata{Loaded: false}
	}

	// Load metadata from file
	if err := t.extractMetadata(); err != nil {
		// Log error but don't fail - fall back to filename
		fmt.Printf("Warning: Failed to extract metadata from %s: %v\n", t.Path, err)
	}

	t.metadata.Loaded = true
}

// extractMetadata extracts metadata from the audio file
func (t *Track) extractMetadata() error {
	// Only support MP3 for now (can expand to other formats later)
	if !strings.EqualFold(t.Format, ".mp3") {
		return fmt.Errorf("metadata extraction not supported for format: %s", t.Format)
	}

	// Open and read metadata
	file, err := os.Open(t.Path)
	if err != nil {
		return fmt.Errorf("failed to open file for metadata: %w", err)
	}
	defer file.Close()

	// Read metadata using tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	// Extract metadata
	if title := metadata.Title(); title != "" {
		t.metadata.Title = title
	}
	if artist := metadata.Artist(); artist != "" {
		t.metadata.Artist = artist
	}
	if album := metadata.Album(); album != "" {
		t.metadata.Album = album
	}
	if genre := metadata.Genre(); genre != "" {
		t.metadata.Genre = genre
	}
	if year := metadata.Year(); year != 0 {
		t.metadata.Year = year
	}
	if track, _ := metadata.Track(); track != 0 {
		t.metadata.TrackNumber = track
	}

	return nil
}

// generateID generates a unique ID for the track based on its path
func generateID(path string) string {
	// Simple hash for now - can be improved with proper hashing if needed
	hash := 0
	for _, char := range path {
		hash = ((hash << 5) - hash + int(char)) & 0xffffffff
	}
	return fmt.Sprintf("%08x", hash)
}

// String returns a string representation of the track
func (t *Track) String() string {
	return fmt.Sprintf("%s (%s)", t.DisplayName(), formatDuration(t.Duration))
}

// formatDuration formats duration in MM:SS format
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
