package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestClaudeEnableProxyMergesExistingSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	initial := `{
  "env": {
    "EXISTING_ENV": "keep",
    "ANTHROPIC_AUTH_TOKEN": "original-token"
  },
  "permissions": {
    "allow": ["Bash(ls)"]
  }
}`
	writeFile(t, settingsPath, initial)

	service := NewClaudeSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	var payload map[string]any
	readJSONFile(t, settingsPath, &payload)

	env, ok := payload["env"].(map[string]any)
	if !ok {
		t.Fatalf("env missing or invalid: %#v", payload["env"])
	}
	if got := env["EXISTING_ENV"]; got != "keep" {
		t.Fatalf("EXISTING_ENV = %#v, want keep", got)
	}
	if got := env["ANTHROPIC_AUTH_TOKEN"]; got != claudeAuthTokenValue {
		t.Fatalf("ANTHROPIC_AUTH_TOKEN = %#v, want %q", got, claudeAuthTokenValue)
	}
	if got := env["ANTHROPIC_BASE_URL"]; got != "http://127.0.0.1:18100" {
		t.Fatalf("ANTHROPIC_BASE_URL = %#v, want local relay", got)
	}
	if _, ok := payload["permissions"]; !ok {
		t.Fatalf("permissions missing after EnableProxy: %#v", payload)
	}
}

func TestClaudeEnableProxyTwiceRestoresOriginalSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	initial := `{"env":{"EXISTING_ENV":"keep","ANTHROPIC_AUTH_TOKEN":"original-token"},"permissions":{"allow":["Bash(ls)"]}}`
	writeFile(t, settingsPath, initial)

	service := NewClaudeSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("first EnableProxy: %v", err)
	}
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("second EnableProxy: %v", err)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("DisableProxy: %v", err)
	}

	assertJSONEqual(t, readFile(t, settingsPath), []byte(initial))
}

func TestClaudeEnableProxyUsesXBackupNameAndToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	initial := `{"env":{"ANTHROPIC_AUTH_TOKEN":"original-token"}}`
	writeFile(t, settingsPath, initial)

	service := NewClaudeSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	assertFileExists(t, filepath.Join(home, ".claude", "code-switch-x.back.settings.json"))
	assertFileNotExists(t, filepath.Join(home, ".claude", ("cc"+"-studio")+".back.settings.json"))

	var payload map[string]any
	readJSONFile(t, settingsPath, &payload)
	env, ok := payload["env"].(map[string]any)
	if !ok {
		t.Fatalf("env missing or invalid: %#v", payload["env"])
	}
	if got := env["ANTHROPIC_AUTH_TOKEN"]; got != "code-switch-x" {
		t.Fatalf("ANTHROPIC_AUTH_TOKEN = %#v, want code-switch-x", got)
	}
}

func TestCodexEnableProxyPreservesAuthFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	authPath := filepath.Join(home, ".codex", "auth.json")
	initialAuth := `{"auth_mode":"chatgpt","OPENAI_API_KEY":"real-key","tokens":{"access_token":"keep-token"},"OTHER_TOKEN":"keep"}`
	writeFile(t, authPath, initialAuth)

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	var payload map[string]any
	readJSONFile(t, authPath, &payload)
	if got := payload["OPENAI_API_KEY"]; got != codexTokenValue {
		t.Fatalf("OPENAI_API_KEY = %q, want %q", got, codexTokenValue)
	}
	if got := payload["auth_mode"]; got != "chatgpt" {
		t.Fatalf("auth_mode = %q, want chatgpt", got)
	}
	if got := payload["OTHER_TOKEN"]; got != "keep" {
		t.Fatalf("OTHER_TOKEN = %q, want keep", got)
	}
	if tokens, ok := payload["tokens"].(map[string]any); !ok || tokens["access_token"] != "keep-token" {
		t.Fatalf("tokens not preserved: %#v", payload["tokens"])
	}
}

func TestCodexEnableProxyDoesNotAddAuthMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	authPath := filepath.Join(home, ".codex", "auth.json")
	initialAuth := `{"OPENAI_API_KEY":"real-key","OTHER_TOKEN":"keep"}`
	writeFile(t, authPath, initialAuth)

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	var payload map[string]any
	readJSONFile(t, authPath, &payload)
	if _, ok := payload["auth_mode"]; ok {
		t.Fatalf("auth_mode should not be added: %#v", payload)
	}
	if got := payload["OPENAI_API_KEY"]; got != codexTokenValue {
		t.Fatalf("OPENAI_API_KEY = %q, want %q", got, codexTokenValue)
	}
	if got := payload["OTHER_TOKEN"]; got != "keep" {
		t.Fatalf("OTHER_TOKEN = %q, want keep", got)
	}
}

func TestCodexEnableProxyPreservesPreferredAuthMethod(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".codex", "config.toml")
	initialConfig := strings.TrimSpace(`
preferred_auth_method = "chatgpt"
model = "custom-model"
`) + "\n"
	writeFile(t, configPath, initialConfig)

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	payload := readTOMLMap(t, configPath)
	if got := payload["preferred_auth_method"]; got != "chatgpt" {
		t.Fatalf("preferred_auth_method = %q, want chatgpt", got)
	}
}

func TestCodexEnableProxySetsPreferredAuthMethodWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".codex", "config.toml")
	initialConfig := strings.TrimSpace(`
model = "custom-model"
`) + "\n"
	writeFile(t, configPath, initialConfig)

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	payload := readTOMLMap(t, configPath)
	if got := payload["preferred_auth_method"]; got != codexPreferredAuth {
		t.Fatalf("preferred_auth_method = %q, want %q", got, codexPreferredAuth)
	}
}

func TestCodexEnableProxyTwiceRestoresOriginalConfigAndAuth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	initialConfig := strings.TrimSpace(`
model = "custom-model"
model_provider = "openai"
preferred_auth_method = "chatgpt"

[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"
wire_api = "responses"
`) + "\n"
	initialAuth := `{"OPENAI_API_KEY":"real-key","OTHER_TOKEN":"keep"}`
	writeFile(t, configPath, initialConfig)
	writeFile(t, authPath, initialAuth)

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("first EnableProxy: %v", err)
	}
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("second EnableProxy: %v", err)
	}
	if err := service.DisableProxy(); err != nil {
		t.Fatalf("DisableProxy: %v", err)
	}

	if got := string(readFile(t, configPath)); got != initialConfig {
		t.Fatalf("config after disable =\n%s\nwant\n%s", got, initialConfig)
	}
	assertJSONEqual(t, readFile(t, authPath), []byte(initialAuth))
}

func TestCodexEnableProxyUsesXProviderTokenAndBackupNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".codex", "config.toml")
	authPath := filepath.Join(home, ".codex", "auth.json")
	writeFile(t, configPath, `model = "custom-model"`+"\n")
	writeFile(t, authPath, `{"OPENAI_API_KEY":"real-key"}`)

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("EnableProxy: %v", err)
	}

	assertFileExists(t, filepath.Join(home, ".codex", "code-switch-x.back.config.toml"))
	assertFileExists(t, filepath.Join(home, ".codex", "code-switch-x.back.auth.json"))
	assertFileNotExists(t, filepath.Join(home, ".codex", ("cc"+"-studio")+".back.config.toml"))
	assertFileNotExists(t, filepath.Join(home, ".codex", ("cc"+"-studio")+".back.auth.json"))

	content := string(readFile(t, configPath))
	if !strings.Contains(content, "[model_providers.code-switch-x]") {
		t.Fatalf("config missing code-switch-x provider:\n%s", content)
	}
	if strings.Contains(content, "[model_providers."+("code"+"-switch")+"]") {
		t.Fatalf("config must not contain legacy provider key:\n%s", content)
	}

	payload := readTOMLMap(t, configPath)
	if got := payload["model_provider"]; got != "code-switch-x" {
		t.Fatalf("model_provider = %q, want code-switch-x", got)
	}

	var auth map[string]any
	readJSONFile(t, authPath, &auth)
	if got := auth["OPENAI_API_KEY"]; got != "code-switch-x" {
		t.Fatalf("OPENAI_API_KEY = %q, want code-switch-x", got)
	}
}

func TestCodexEnableProxyDoesNotDuplicateCodeSwitchXProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".codex", "config.toml")
	writeFile(t, configPath, `model = "custom-model"`+"\n")

	service := NewCodexSettingsService(":18100")
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("first EnableProxy: %v", err)
	}
	if err := service.EnableProxy(); err != nil {
		t.Fatalf("second EnableProxy: %v", err)
	}

	content := string(readFile(t, configPath))
	if count := strings.Count(content, "[model_providers.code-switch-x]"); count != 1 {
		t.Fatalf("code-switch-x provider count = %d, want 1\n%s", count, content)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to not exist, stat error: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	if err := json.Unmarshal(readFile(t, path), target); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}

func readTOMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := toml.Unmarshal(readFile(t, path), &payload); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return payload
}

func assertJSONEqual(t *testing.T, got []byte, want []byte) {
	t.Helper()
	var gotPayload any
	var wantPayload any
	if err := json.Unmarshal(got, &gotPayload); err != nil {
		t.Fatalf("got invalid JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &wantPayload); err != nil {
		t.Fatalf("want invalid JSON: %v\n%s", err, want)
	}
	gotCanonical, _ := json.Marshal(gotPayload)
	wantCanonical, _ := json.Marshal(wantPayload)
	if string(gotCanonical) != string(wantCanonical) {
		t.Fatalf("JSON mismatch\ngot  %s\nwant %s", gotCanonical, wantCanonical)
	}
}
