package token

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

const (
	cacheDirPerm  = 0700
	tokenFilePerm = 0600
)

// TokenData is the on-disk representation of a cached OAuth token.
type TokenData struct {
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token,omitempty"`
	ExpirationTime time.Time `json:"expiration_time"`
}

// Storage handles atomic token file operations under ~/.cache/mcp-proxy/.
type Storage struct {
	cacheDir string
}

// NewStorage returns a Storage rooted at the user's cache directory.
func NewStorage() (*Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, apperr.NewFileSystemError("failed to get user home directory", err)
	}
	return &Storage{cacheDir: filepath.Join(homeDir, ".cache", "mcp-proxy")}, nil
}

// CacheDir returns the absolute cache directory path.
func (s *Storage) CacheDir() string {
	return s.cacheDir
}

// GetCacheDir is kept for backwards compatibility.
//
// Deprecated: use CacheDir.
func (s *Storage) GetCacheDir() string {
	return s.cacheDir
}

// Save writes a token to disk atomically (temp file + rename) with 0600 perms.
func (s *Storage) Save(serverURL string, accessToken string, refreshToken string, expiresIn int) error {
	if err := s.ensureCacheDir(); err != nil {
		return err
	}

	filename := s.tokenFilePath(serverURL)
	expirationTime := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)

	// #nosec G117 -- this struct is the on-disk token record; serialising secrets is its purpose.
	data, err := json.MarshalIndent(TokenData{
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
		ExpirationTime: expirationTime,
	}, "", "  ")
	if err != nil {
		return apperr.NewFileSystemError("failed to marshal token data", err)
	}

	tempFile := filename + ".tmp"
	if err := os.WriteFile(tempFile, data, tokenFilePerm); err != nil {
		return apperr.NewFileSystemError(fmt.Sprintf("Cannot save tokens to %s", filename), err)
	}
	if err := os.Rename(tempFile, filename); err != nil {
		_ = os.Remove(tempFile)
		return apperr.NewFileSystemError(fmt.Sprintf("Cannot save tokens to %s", filename), err)
	}
	return nil
}

// Load reads and validates a token file. Returns apperr.ErrTokenFileNotFound
// (wrapped) when no file exists, apperr.ErrInvalidTokenFormat when the file
// cannot be parsed, and apperr.ErrTokenMissingField when required fields
// are absent.
func (s *Storage) Load(serverURL string) (*TokenData, error) {
	filename := s.tokenFilePath(serverURL)

	// #nosec G304 -- filename is derived from a base64-encoded server URL within the user's cache dir.
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperr.ErrTokenFileNotFound
		}
		return nil, apperr.NewFileSystemError("failed to read token file", err)
	}

	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, apperr.ErrInvalidTokenFormat
	}

	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("%w: access_token", apperr.ErrTokenMissingField)
	}
	if tokenData.ExpirationTime.IsZero() {
		return nil, fmt.Errorf("%w: expiration_time", apperr.ErrTokenMissingField)
	}

	return &tokenData, nil
}

func (s *Storage) ensureCacheDir() error {
	info, err := os.Stat(s.cacheDir)
	if err == nil {
		if !info.IsDir() {
			return apperr.NewFileSystemError(
				fmt.Sprintf("Cannot create token cache directory at %s: path exists but is not a directory", s.cacheDir),
				nil,
			)
		}
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(s.cacheDir, cacheDirPerm); err != nil {
			return apperr.NewFileSystemError(
				fmt.Sprintf("Cannot create token cache directory at %s", s.cacheDir),
				err,
			)
		}
		return nil
	}
	return apperr.NewFileSystemError(
		fmt.Sprintf("Cannot create token cache directory at %s", s.cacheDir),
		err,
	)
}

func (s *Storage) tokenFilePath(serverURL string) string {
	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(serverURL))
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "+", "-")
	return filepath.Join(s.cacheDir, encoded+".json")
}
