package token_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/smorand/mcp-proxy/internal/apperr"
	"github.com/smorand/mcp-proxy/internal/token"
)

func setupStorage(t *testing.T) *token.Storage {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	storage, err := token.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	return storage
}

func TestStorage_SaveAndLoad(t *testing.T) {
	storage := setupStorage(t)
	const serverURL = "https://mcp.example.com"

	if err := storage.Save(serverURL, "AT-1", "RT-1", 3600); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := storage.Load(serverURL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "AT-1" {
		t.Errorf("AccessToken = %q", got.AccessToken)
	}
	if got.RefreshToken != "RT-1" {
		t.Errorf("RefreshToken = %q", got.RefreshToken)
	}
}

func TestStorage_FilePermissions(t *testing.T) {
	storage := setupStorage(t)
	const serverURL = "https://mcp.example.com"

	if err := storage.Save(serverURL, "AT", "RT", 3600); err != nil {
		t.Fatalf("Save: %v", err)
	}

	dirInfo, err := os.Stat(storage.CacheDir())
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("dir perms = %o, want 0700", dirInfo.Mode().Perm())
	}

	entries, err := os.ReadDir(storage.CacheDir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatalf("Info: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("file %s perms = %o, want 0600", e.Name(), info.Mode().Perm())
		}
	}
}

func TestStorage_LoadNotFound(t *testing.T) {
	storage := setupStorage(t)
	_, err := storage.Load("https://missing.example.com")
	if !errors.Is(err, apperr.ErrTokenFileNotFound) {
		t.Errorf("err = %v, want ErrTokenFileNotFound", err)
	}
}

func TestStorage_LoadInvalidJSON(t *testing.T) {
	storage := setupStorage(t)
	if err := os.MkdirAll(storage.CacheDir(), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const serverURL = "https://corrupt.example.com"
	if err := storage.Save(serverURL, "AT", "RT", 3600); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, _ := os.ReadDir(storage.CacheDir())
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			path := filepath.Join(storage.CacheDir(), e.Name())
			if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}
	}

	_, err := storage.Load(serverURL)
	if !errors.Is(err, apperr.ErrInvalidTokenFormat) {
		t.Errorf("err = %v, want ErrInvalidTokenFormat", err)
	}
}

func TestStorage_LoadMissingField(t *testing.T) {
	storage := setupStorage(t)
	if err := os.MkdirAll(storage.CacheDir(), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const serverURL = "https://incomplete.example.com"
	if err := storage.Save(serverURL, "AT", "", 3600); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, _ := os.ReadDir(storage.CacheDir())
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			path := filepath.Join(storage.CacheDir(), e.Name())
			if err := os.WriteFile(path, []byte(`{"refresh_token":"RT"}`), 0600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}
	}

	_, err := storage.Load(serverURL)
	if !errors.Is(err, apperr.ErrTokenMissingField) {
		t.Errorf("err = %v, want ErrTokenMissingField", err)
	}
}
