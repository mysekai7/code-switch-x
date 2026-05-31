package services

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

const (
	appSettingsFile       = "app.json"
	defaultRelayPort      = 18101
	DefaultRelayPort      = defaultRelayPort
	defaultRawLogMaxBytes = 262144
	maxFallbackAttempts   = 50
	minRelayPort          = 1024
	maxRelayPort          = 65535
)

type AppSettings struct {
	ShowHeatmap                 bool `json:"show_heatmap"`
	ShowHomeTitle               bool `json:"show_home_title"`
	AutoStart                   bool `json:"auto_start"`
	RelayPort                   int  `json:"relay_port"`
	CaptureRawLogs              bool `json:"capture_raw_logs"`
	RawLogMaxBytes              int  `json:"raw_log_max_bytes"`
	ClaudeThinkingRectifier     bool `json:"claude_thinking_rectifier"`
	ProviderFallbackEnabled     bool `json:"provider_fallback_enabled"`
	ProviderFallbackMaxAttempts int  `json:"provider_fallback_max_attempts"`
}

type AppSettingsService struct {
	path             string
	mu               sync.Mutex
	autoStartService *AutoStartService
}

func NewAppSettingsService(autoStartService *AutoStartService) *AppSettingsService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	path := filepath.Join(home, appSettingsDirName, appSettingsFile)
	return &AppSettingsService{
		path:             path,
		autoStartService: autoStartService,
	}
}

func (as *AppSettingsService) defaultSettings() AppSettings {
	// 检查当前开机自启动状态
	autoStartEnabled := false
	if as.autoStartService != nil {
		if enabled, err := as.autoStartService.IsEnabled(); err == nil {
			autoStartEnabled = enabled
		}
	}

	return AppSettings{
		ShowHeatmap:                 true,
		ShowHomeTitle:               true,
		AutoStart:                   autoStartEnabled,
		RelayPort:                   defaultRelayPort,
		CaptureRawLogs:              false,
		RawLogMaxBytes:              defaultRawLogMaxBytes,
		ClaudeThinkingRectifier:     true,
		ProviderFallbackEnabled:     true,
		ProviderFallbackMaxAttempts: 0,
	}
}

// GetAppSettings returns the persisted app settings or defaults if the file does not exist.
func (as *AppSettingsService) GetAppSettings() (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.loadLocked()
}

// SaveAppSettings persists the provided settings to disk.
func (as *AppSettingsService) SaveAppSettings(settings AppSettings) (AppSettings, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	settings.RawLogMaxBytes = normalizeRawLogMaxBytes(settings.RawLogMaxBytes)
	settings.ProviderFallbackMaxAttempts = normalizeProviderFallbackMaxAttempts(settings.ProviderFallbackMaxAttempts)
	if err := validateRelayPort(settings.RelayPort); err != nil {
		return settings, err
	}
	currentSettings, err := as.loadLocked()
	if err != nil {
		return settings, err
	}
	if settings.RelayPort != currentSettings.RelayPort {
		if err := validateRelayPortAvailable(settings.RelayPort); err != nil {
			return settings, err
		}
	}

	// 同步开机自启动状态
	if as.autoStartService != nil {
		if settings.AutoStart {
			if err := as.autoStartService.Enable(); err != nil {
				return settings, err
			}
		} else {
			if err := as.autoStartService.Disable(); err != nil {
				return settings, err
			}
		}
	}

	if err := as.saveLocked(settings); err != nil {
		return settings, err
	}
	return settings, nil
}

func (as *AppSettingsService) loadLocked() (AppSettings, error) {
	settings := as.defaultSettings()
	data, err := os.ReadFile(as.path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return settings, err
	}
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	return normalizeAppSettings(settings), nil
}

func normalizeAppSettings(settings AppSettings) AppSettings {
	if settings.RelayPort == 0 {
		settings.RelayPort = defaultRelayPort
	}
	if settings.RawLogMaxBytes <= 0 {
		settings.RawLogMaxBytes = defaultRawLogMaxBytes
	}
	if settings.ProviderFallbackMaxAttempts < 0 {
		settings.ProviderFallbackMaxAttempts = 0
	}
	if settings.ProviderFallbackMaxAttempts > maxFallbackAttempts {
		settings.ProviderFallbackMaxAttempts = maxFallbackAttempts
	}
	return settings
}

func normalizeProviderFallbackMaxAttempts(attempts int) int {
	if attempts < 0 {
		return 0
	}
	if attempts > maxFallbackAttempts {
		return maxFallbackAttempts
	}
	return attempts
}

func (as *AppSettingsService) saveLocked(settings AppSettings) error {
	dir := filepath.Dir(as.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(as.path, data, 0o644)
}

func validateRelayPort(port int) error {
	if port < minRelayPort || port > maxRelayPort {
		return fmt.Errorf("代理端口必须在 %d-%d 之间", minRelayPort, maxRelayPort)
	}
	return nil
}

func validateRelayPortAvailable(port int) error {
	addresses := []struct {
		network string
		address string
	}{
		{"tcp4", fmt.Sprintf("127.0.0.1:%d", port)},
		{"tcp6", fmt.Sprintf("[::1]:%d", port)},
	}
	for _, target := range addresses {
		listener, err := net.Listen(target.network, target.address)
		if err != nil {
			return fmt.Errorf("代理端口 %d 已被占用", port)
		}
		if err := listener.Close(); err != nil {
			return err
		}
	}
	return nil
}
