package services

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppSettingsDefaultsRelayPortTo18101(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	service := NewAppSettingsService(nil)
	settings, err := service.GetAppSettings()
	if err != nil {
		t.Fatalf("GetAppSettings() error = %v", err)
	}

	if settings.RelayPort != defaultRelayPort {
		t.Fatalf("RelayPort = %d, want %d", settings.RelayPort, defaultRelayPort)
	}
}

func TestAppSettingsDefaultsRawLogCaptureOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	service := NewAppSettingsService(nil)
	settings, err := service.GetAppSettings()
	if err != nil {
		t.Fatalf("GetAppSettings() error = %v", err)
	}
	if settings.CaptureRawLogs {
		t.Fatalf("CaptureRawLogs = true, want false")
	}
	if settings.RawLogMaxBytes != defaultRawLogMaxBytes {
		t.Fatalf("RawLogMaxBytes = %d, want %d", settings.RawLogMaxBytes, defaultRawLogMaxBytes)
	}
}

func TestAppSettingsPersistsRelayPort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	service := NewAppSettingsService(nil)
	settings, err := service.SaveAppSettings(AppSettings{
		ShowHeatmap:   true,
		ShowHomeTitle: true,
		AutoStart:     false,
		RelayPort:     18111,
	})
	if err != nil {
		t.Fatalf("SaveAppSettings() error = %v", err)
	}
	if settings.RelayPort != 18111 {
		t.Fatalf("saved RelayPort = %d, want 18111", settings.RelayPort)
	}

	data, err := os.ReadFile(filepath.Join(home, appSettingsDirName, appSettingsFile))
	if err != nil {
		t.Fatalf("ReadFile(app settings) error = %v", err)
	}
	if !strings.Contains(string(data), `"relay_port": 18111`) {
		t.Fatalf("settings file missing relay_port:\n%s", data)
	}
}

func TestAppSettingsRejectsInvalidRelayPort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	service := NewAppSettingsService(nil)

	for _, port := range []int{0, 1023, 65536} {
		_, err := service.SaveAppSettings(AppSettings{RelayPort: port})
		if err == nil {
			t.Fatalf("SaveAppSettings(RelayPort=%d) expected error", port)
		}
	}
}

func TestAppSettingsRejectsBusyRelayPortWhenChanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	service := NewAppSettingsService(nil)
	_, err = service.SaveAppSettings(AppSettings{RelayPort: port})
	if err == nil {
		t.Fatalf("SaveAppSettings(RelayPort=%d) expected busy-port error", port)
	}
}

func TestRelayServicesDefaultTo18101(t *testing.T) {
	if got := NewProviderRelayService(NewProviderService(), "").Addr(); got != ":18101" {
		t.Fatalf("ProviderRelayService Addr() = %q, want :18101", got)
	}
	if got := NewClaudeSettingsService("").baseURL(); got != "http://127.0.0.1:18101" {
		t.Fatalf("Claude baseURL() = %q, want http://127.0.0.1:18101", got)
	}
	if got := NewCodexSettingsService("").baseURL(); got != "http://127.0.0.1:18101" {
		t.Fatalf("Codex baseURL() = %q, want http://127.0.0.1:18101", got)
	}
}
