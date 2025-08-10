package playlist

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"perth/player"
)

// Scanner manages the scanning and caching of audio files
type Scanner struct {
	cachePath  string            // JSON cache file path
	tracks     []*Track          // In-memory track list
	lastScan   time.Time         // Last scan timestamp
	fileHashes map[string]string // Path -> hash for change detection

	// Configuration
	scanPaths  []string        // Directories to scan
	extensions map[string]bool // Supported audio extensions
}

// ScanResult contains the result of a scan operation
type ScanResult struct {
	Tracks        []*Track  `json:"tracks"`
	TotalFiles    int       `json:"total_files"`
	NewTracks     int       `json:"new_tracks"`
	UpdatedTracks int       `json:"updated_tracks"`
	RemovedTracks int       `json:"removed_tracks"`
	Errors        []string  `json:"errors,omitempty"`
	ScanTime      time.Time `json:"scan_time"`
}

// NewScanner creates a new Scanner instance
func NewScanner(scanPaths []string) *Scanner {
	if len(scanPaths) == 0 {
		scanPaths = []string{"assets"}
	}

	// Determine cache path (global vs local)
	cachePath := getCachePath()

	return &Scanner{
		cachePath:  cachePath,
		tracks:     []*Track{},
		fileHashes: make(map[string]string),
		scanPaths:  scanPaths,
		extensions: map[string]bool{
			".mp3":  true,
			".wav":  true,
			".flac": true,
		},
	}
}

// Scan performs a full scan of the configured directories
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		ScanTime: time.Now(),
		Errors:   []string{},
	}

	// Load existing cache first
	if err := s.loadCache(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to load cache: %v", err))
	}

	// Scan all configured paths
	for _, path := range s.scanPaths {
		if err := s.scanDirectory(path, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to scan %s: %v", path, err))
		}
	}

	// Remove tracks that no longer exist
	s.removeDeletedTracks(result)

	// Save updated cache
	if err := s.saveCache(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to save cache: %v", err))
	}

	result.Tracks = s.tracks
	result.TotalFiles = len(s.tracks)
	result.ScanTime = time.Now()

	return result, nil
}

// IncrementalScan performs a quick scan to check for changes
func (s *Scanner) IncrementalScan() (*ScanResult, error) {
	result := &ScanResult{
		ScanTime: time.Now(),
		Errors:   []string{},
	}

	// Load existing cache
	if err := s.loadCache(); err != nil {
		// If cache is corrupted, fall back to full scan
		return s.Scan()
	}

	// Check for changes in existing files
	changed := false
	for _, track := range s.tracks {
		if s.hasFileChanged(track.Path) {
			// File changed, need to rescan
			changed = true
			break
		}
	}

	if changed {
		// Perform full scan if changes detected
		return s.Scan()
	}

	// Check for new files
	newTracks := s.scanForNewFiles(result)
	if newTracks > 0 {
		// Save updated cache
		if err := s.saveCache(); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to save cache: %v", err))
		}
	}

	result.Tracks = s.tracks
	result.TotalFiles = len(s.tracks)
	result.NewTracks = newTracks
	result.ScanTime = time.Now()

	return result, nil
}

// scanDirectory scans a single directory for audio files
func (s *Scanner) scanDirectory(dirPath string, result *ScanResult) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively scan subdirectories
			subPath := filepath.Join(dirPath, entry.Name())
			if err := s.scanDirectory(subPath, result); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to scan subdirectory %s: %v", subPath, err))
			}
			continue
		}

		// Check if it's an audio file
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !s.extensions[ext] {
			continue
		}

		// Process audio file
		if err := s.processAudioFile(dirPath, entry, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to process %s: %v", entry.Name(), err))
		}
	}

	return nil
}

// processAudioFile processes a single audio file
func (s *Scanner) processAudioFile(dirPath string, entry os.DirEntry, result *ScanResult) error {
	fullPath := filepath.Join(dirPath, entry.Name())

	// Check if file already exists in cache
	existingTrack := s.findTrackByPath(fullPath)
	if existingTrack != nil {
		// Check if file has changed
		if !s.hasFileChanged(fullPath) {
			return nil // No changes
		}

		// File changed, update track
		if err := s.updateTrack(existingTrack, fullPath); err != nil {
			return fmt.Errorf("failed to update track: %w", err)
		}
		result.UpdatedTracks++
		return nil
	}

	// New file, create track
	track, err := s.createTrack(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create track: %w", err)
	}

	s.tracks = append(s.tracks, track)
	result.NewTracks++

	return nil
}

// createTrack creates a new Track from a file path
func (s *Scanner) createTrack(filePath string) (*Track, error) {
	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Get duration using player decoder
	duration, err := s.getAudioDuration(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get duration: %w", err)
	}

	// Create track
	track := NewTrack(filePath, duration, info.Size(), info.ModTime())

	// Store file hash for change detection
	s.fileHashes[filePath] = s.calculateFileHash(filePath)

	return track, nil
}

// updateTrack updates an existing track with new file information
func (s *Scanner) updateTrack(track *Track, filePath string) error {
	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Get duration
	duration, err := s.getAudioDuration(filePath)
	if err != nil {
		return fmt.Errorf("failed to get duration: %w", err)
	}

	// Update track
	track.Duration = duration
	track.Size = info.Size()
	track.Modified = info.ModTime()

	// Reset metadata to force reload
	track.metadata.Loaded = false

	// Update file hash
	s.fileHashes[filePath] = s.calculateFileHash(filePath)

	return nil
}

// getAudioDuration gets the duration of an audio file
func (s *Scanner) getAudioDuration(filePath string) (time.Duration, error) {
	// Use player decoder to get duration
	stream, format, err := player.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer stream.Close()

	// Get duration if available
	type posLen interface {
		Position() int
		Len() int
	}

	if s, ok := stream.(posLen); ok && s.Len() > 0 {
		return format.SampleRate.D(s.Len()), nil
	}

	return 0, nil
}

// hasFileChanged checks if a file has changed since last scan
func (s *Scanner) hasFileChanged(filePath string) bool {
	currentHash := s.calculateFileHash(filePath)
	lastHash, exists := s.fileHashes[filePath]

	if !exists {
		return true // New file
	}

	return currentHash != lastHash
}

// calculateFileHash calculates MD5 hash of a file
func (s *Scanner) calculateFileHash(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

// findTrackByPath finds a track by its file path
func (s *Scanner) findTrackByPath(filePath string) *Track {
	for _, track := range s.tracks {
		if track.Path == filePath {
			return track
		}
	}
	return nil
}

// removeDeletedTracks removes tracks for files that no longer exist
func (s *Scanner) removeDeletedTracks(result *ScanResult) {
	var remainingTracks []*Track

	for _, track := range s.tracks {
		if _, err := os.Stat(track.Path); os.IsNotExist(err) {
			// File deleted, remove from cache
			delete(s.fileHashes, track.Path)
			result.RemovedTracks++
		} else {
			remainingTracks = append(remainingTracks, track)
		}
	}

	s.tracks = remainingTracks
}

// scanForNewFiles scans for new files that weren't in the cache
func (s *Scanner) scanForNewFiles(result *ScanResult) int {
	newCount := 0

	for _, path := range s.scanPaths {
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if !s.extensions[ext] {
				continue
			}

			fullPath := filepath.Join(path, entry.Name())
			if s.findTrackByPath(fullPath) == nil {
				// New file found
				if track, err := s.createTrack(fullPath); err == nil {
					s.tracks = append(s.tracks, track)
					newCount++
				}
			}
		}
	}

	return newCount
}

// GetTracks returns all tracks in the scanner
func (s *Scanner) GetTracks() []*Track {
	return s.tracks
}

// GetTrackByID returns a track by its ID
func (s *Scanner) GetTrackByID(id string) *Track {
	for _, track := range s.tracks {
		if track.ID == id {
			return track
		}
	}
	return nil
}

// GetTrackByPath returns a track by its file path
func (s *Scanner) GetTrackByPath(path string) *Track {
	return s.findTrackByPath(path)
}

// loadCache loads the track cache from disk
func (s *Scanner) loadCache() error {
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file, start fresh
		}
		return fmt.Errorf("failed to read cache: %w", err)
	}

	var cache struct {
		Tracks     []*Track          `json:"tracks"`
		FileHashes map[string]string `json:"file_hashes"`
		LastScan   time.Time         `json:"last_scan"`
	}

	if err := json.Unmarshal(data, &cache); err != nil {
		return fmt.Errorf("failed to parse cache: %w", err)
	}

	s.tracks = cache.Tracks
	s.fileHashes = cache.FileHashes
	s.lastScan = cache.LastScan

	return nil
}

// saveCache saves the track cache to disk
func (s *Scanner) saveCache() error {
	// Ensure cache directory exists
	cacheDir := filepath.Dir(s.cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := struct {
		Tracks     []*Track          `json:"tracks"`
		FileHashes map[string]string `json:"file_hashes"`
		LastScan   time.Time         `json:"last_scan"`
	}{
		Tracks:     s.tracks,
		FileHashes: s.fileHashes,
		LastScan:   time.Now(),
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(s.cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	return nil
}

// getCachePath determines the cache file path (global vs local)
func getCachePath() string {
	// Check if we're in a project directory
	if _, err := os.Stat("go.mod"); err == nil {
		// Project directory, use local cache
		return ".perth/cache.json"
	}

	// Use global cache
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		return ".perth/cache.json"
	}

	return filepath.Join(homeDir, ".perth", "cache.json")
}
