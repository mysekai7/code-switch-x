package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudeSettingsDir      = ".claude"
	claudeSettingsFileName = "settings.json"
	claudeBackupFileName   = "code-switch-x.back.settings.json"
	claudeAuthTokenValue   = "code-switch-x"
)

type ClaudeProxyStatus struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"`
}

type ClaudeSettingsService struct {
	relayAddr string
}

func NewClaudeSettingsService(relayAddr string) *ClaudeSettingsService {
	return &ClaudeSettingsService{relayAddr: relayAddr}
}

func (css *ClaudeSettingsService) ProxyStatus() (ClaudeProxyStatus, error) {
	status := ClaudeProxyStatus{Enabled: false, BaseURL: css.baseURL()}
	settingsPath, _, err := css.paths()
	if err != nil {
		return status, err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status, nil
		}
		return status, err
	}
	var payload claudeSettingsFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return status, nil
	}
	baseURL := css.baseURL()
	enabled := strings.EqualFold(payload.Env["ANTHROPIC_AUTH_TOKEN"], claudeAuthTokenValue) &&
		strings.EqualFold(payload.Env["ANTHROPIC_BASE_URL"], baseURL)
	status.Enabled = enabled
	return status, nil
}

func (css *ClaudeSettingsService) EnableProxy() error {
	settingsPath, backupPath, err := css.paths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	settings := make(map[string]any)
	if _, err := os.Stat(settingsPath); err == nil {
		content, readErr := os.ReadFile(settingsPath)
		if readErr != nil {
			return readErr
		}
		if _, backupErr := os.Stat(backupPath); errors.Is(backupErr, os.ErrNotExist) {
			if err := os.WriteFile(backupPath, content, 0o600); err != nil {
				return err
			}
		} else if backupErr != nil {
			return backupErr
		}
		if len(content) > 0 {
			if err := json.Unmarshal(content, &settings); err != nil {
				return err
			}
		}
	}
	env := ensureJSONTable(settings, "env")
	env["ANTHROPIC_AUTH_TOKEN"] = claudeAuthTokenValue
	env["ANTHROPIC_BASE_URL"] = css.baseURL()

	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, payload, 0o600)
}

func ensureJSONTable(payload map[string]any, key string) map[string]any {
	if value, ok := payload[key]; ok {
		if table, ok := value.(map[string]any); ok {
			return table
		}
	}
	table := make(map[string]any)
	payload[key] = table
	return table
}

func (css *ClaudeSettingsService) DisableProxy() error {
	settingsPath, backupPath, err := css.paths()
	if err != nil {
		return err
	}
	if err := os.Remove(settingsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Rename(backupPath, settingsPath); err != nil {
			return err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return nil
}

func (css *ClaudeSettingsService) paths() (settingsPath string, backupPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, claudeSettingsDir)
	return filepath.Join(dir, claudeSettingsFileName), filepath.Join(dir, claudeBackupFileName), nil
}

func (css *ClaudeSettingsService) baseURL() string {
	addr := strings.TrimSpace(css.relayAddr)
	if addr == "" {
		addr = ":18100"
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return host
}

type claudeSettingsFile struct {
	Env map[string]string `json:"env"`
}
