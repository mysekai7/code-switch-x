package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProviderRelayServiceCreatesAppDataDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	NewProviderRelayService(NewProviderService(), ":0")

	if _, err := os.Stat(filepath.Join(home, appDataDirName)); err != nil {
		t.Fatalf("app data directory was not created: %v", err)
	}
}
