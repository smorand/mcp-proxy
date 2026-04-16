package token

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// TokenData represents the token information stored in the cache
type TokenData struct {
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token,omitempty"`
	ExpirationTime time.Time `json:"expiration_time"`
}

// Storage handles token file operations
type Storage struct {
	cacheDir string
}

// NewStorage creates a new token storage instance
func NewStorage() (*Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.NewFileSystemError("failed to get user home directory", err)
	}

	cacheDir := filepath.Join(homeDir, ".cache", "mcp-proxy")
	return &Storage{cacheDir: cacheDir}, nil
}

// Save saves token data to disk with proper permissions
func (s *Storage) Save(serverURL string, accessToken string, refreshToken string, expiresIn int) error {
	// Ensure cache directory exists with proper permissions
	if err := s.ensureCacheDir(); err != nil {
		return err
	}

	// Generate filename from server URL
	filename := s.getTokenFilePath(serverURL)

	// Compute expiration time
	expirationTime := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)

	// Create token data
	tokenData := TokenData{
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
		ExpirationTime: expirationTime,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return errors.NewFileSystemError("failed to marshal token data", err)
	}

	// Write atomically: write to temp file, then rename
	tempFile := filename + ".tmp"

	// Write to temp file with 0600 permissions
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return errors.NewFileSystemError(
			fmt.Sprintf("Cannot save tokens to %s", filename),
			err,
		)
	}

	// Rename temp file to final filename (atomic operation)
	if err := os.Rename(tempFile, filename); err != nil {
		// Clean up temp file on error
		os.Remove(tempFile)
		return errors.NewFileSystemError(
			fmt.Sprintf("Cannot save tokens to %s", filename),
			err,
		)
	}

	return nil
}

// Load loads token data from disk
func (s *Storage) Load(serverURL string) (*TokenData, error) {
	filename := s.getTokenFilePath(serverURL)

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("token file not found")
		}
		return nil, errors.NewFileSystemError("failed to read token file", err)
	}

	// Parse JSON
	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, fmt.Errorf("invalid token file format")
	}

	// Validate required fields
	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("token file missing access_token")
	}
	if tokenData.ExpirationTime.IsZero() {
		return nil, fmt.Errorf("token file missing expiration_time")
	}

	return &tokenData, nil
}

// ensureCacheDir creates the cache directory if it doesn't exist
func (s *Storage) ensureCacheDir() error {
	// Check if directory exists
	info, err := os.Stat(s.cacheDir)
	if err == nil {
		// Directory exists, check if it's actually a directory
		if !info.IsDir() {
			return errors.NewFileSystemError(
				fmt.Sprintf("Cannot create token cache directory at %s: path exists but is not a directory", s.cacheDir),
				nil,
			)
		}
		return nil
	}

	// Directory doesn't exist, create it
	if os.IsNotExist(err) {
		if err := os.MkdirAll(s.cacheDir, 0700); err != nil {
			return errors.NewFileSystemError(
				fmt.Sprintf("Cannot create token cache directory at %s", s.cacheDir),
				err,
			)
		}
		return nil
	}

	// Other error
	return errors.NewFileSystemError(
		fmt.Sprintf("Cannot create token cache directory at %s", s.cacheDir),
		err,
	)
}

// getTokenFilePath generates the token file path for a given server URL
func (s *Storage) getTokenFilePath(serverURL string) string {
	// Base64url encode the URL (RFC 4648, URL-safe alphabet, no padding)
	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(serverURL))

	// Replace any remaining problematic characters
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "+", "-")

	filename := encoded + ".json"
	return filepath.Join(s.cacheDir, filename)
}

// GetCacheDir returns the cache directory path (for testing)
func (s *Storage) GetCacheDir() string {
	return s.cacheDir
}
