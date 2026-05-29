package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if got := payload["auth_mode"]; got != "apikey" {
		t.Fatalf("auth_mode = %q, want apikey", got)
	}
	if got := payload["OTHER_TOKEN"]; got != "keep" {
		t.Fatalf("OTHER_TOKEN = %q, want keep", got)
	}
	if tokens, ok := payload["tokens"].(map[string]any); !ok || tokens["access_token"] != "keep-token" {
		t.Fatalf("tokens not preserved: %#v", payload["tokens"])
	}
}

func TestCodexEnableProxyStripsPreferredAuthMethod(t *testing.T) {
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

	content := string(readFile(t, configPath))
	if strings.Contains(content, "preferred_auth_method") {
		t.Fatalf("preferred_auth_method should be stripped from written config:\n%s", content)
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

func TestCodexEnableProxyDoesNotDuplicateCodeSwitchProvider(t *testing.T) {
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
	if count := strings.Count(content, "[model_providers.code-switch]"); count != 1 {
		t.Fatalf("code-switch provider count = %d, want 1\n%s", count, content)
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

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	if err := json.Unmarshal(readFile(t, path), target); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
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
