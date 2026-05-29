package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderFilePathUsesXAppDataDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path, err := providerFilePath("claude")
	if err != nil {
		t.Fatalf("providerFilePath() error = %v", err)
	}

	if !strings.Contains(path, string(filepath.Separator)+".code-switch-x"+string(filepath.Separator)) {
		t.Fatalf("provider path = %q, want app-owned .code-switch-x directory", path)
	}
	if strings.Contains(path, string(filepath.Separator)+".code-switch"+string(filepath.Separator)) {
		t.Fatalf("provider path = %q, must not use legacy .code-switch directory", path)
	}
}

func TestAppSettingsServiceUsesXSettingsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	service := NewAppSettingsService(nil)

	if !strings.Contains(service.path, string(filepath.Separator)+".codex-switch-x"+string(filepath.Separator)) {
		t.Fatalf("app settings path = %q, want app-owned .codex-switch-x directory", service.path)
	}
	if strings.Contains(service.path, string(filepath.Separator)+".codex-switch"+string(filepath.Separator)) {
		t.Fatalf("app settings path = %q, must not use legacy .codex-switch directory", service.path)
	}
}

func TestAutoStartServiceUsesXIdentifiers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	service := NewAutoStartService()

	darwinPath := service.getDarwinPlistPath()
	if !strings.HasSuffix(darwinPath, filepath.Join("Library", "LaunchAgents", "com.codeswitch-x.app.plist")) {
		t.Fatalf("darwin plist path = %q, want com.codeswitch-x.app.plist", darwinPath)
	}
	if strings.HasSuffix(darwinPath, filepath.Join("Library", "LaunchAgents", "com.codeswitch.app.plist")) {
		t.Fatalf("darwin plist path = %q, must not use legacy bundle identifier", darwinPath)
	}

	if err := service.enableDarwin(); err != nil {
		t.Fatalf("enableDarwin() error = %v", err)
	}
	plist, err := os.ReadFile(darwinPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", darwinPath, err)
	}
	if !strings.Contains(string(plist), "<string>com.codeswitch-x.app</string>") {
		t.Fatalf("darwin plist content = %q, want x bundle identifier label", string(plist))
	}
	if strings.Contains(string(plist), "<string>com.codeswitch.app</string>") {
		t.Fatalf("darwin plist content = %q, must not use legacy bundle identifier label", string(plist))
	}

	linuxPath := service.getLinuxDesktopPath()
	if !strings.HasSuffix(linuxPath, filepath.Join("autostart", "codeswitch-x.desktop")) {
		t.Fatalf("linux desktop path = %q, want codeswitch-x.desktop", linuxPath)
	}
	if strings.HasSuffix(linuxPath, filepath.Join("autostart", "codeswitch.desktop")) {
		t.Fatalf("linux desktop path = %q, must not use legacy desktop filename", linuxPath)
	}
	if err := service.enableLinux(); err != nil {
		t.Fatalf("enableLinux() error = %v", err)
	}
	desktop, err := os.ReadFile(linuxPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", linuxPath, err)
	}
	if !strings.Contains(string(desktop), "Name=Code Switch X\n") {
		t.Fatalf("linux desktop content = %q, want display name", string(desktop))
	}
}

func TestBuildMetadataUsesXApplicationIdentity(t *testing.T) {
	files := map[string][]string{
		filepath.Join("..", "build", "config.yml"): {
			`productName: "Code Switch X"`,
			`productIdentifier: "com.codeswitch-x.app"`,
			`comments: "Code Switch X desktop relay controller"`,
		},
		filepath.Join("..", "build", "darwin", "Info.plist"): {
			"<string>Code Switch X</string>",
			"<string>CodeSwitchX</string>",
			"<string>com.codeswitch-x.app</string>",
			"<string>Code Switch X desktop relay controller</string>",
		},
		filepath.Join("..", "build", "darwin", "Info.dev.plist"): {
			"<string>Code Switch X</string>",
			"<string>CodeSwitchX</string>",
			"<string>com.codeswitch-x.app</string>",
			"<string>Code Switch X desktop relay controller</string>",
		},
		filepath.Join("..", "build", "windows", "info.json"): {
			`"ProductName": "Code Switch X"`,
			`"Comments": "Code Switch X desktop relay controller"`,
		},
		filepath.Join("..", "build", "linux", "CodeSwitchX.desktop"): {
			"Name=Code Switch X",
			"Exec=/usr/local/bin/CodeSwitchX %u",
			"StartupWMClass=CodeSwitchX",
		},
	}

	for path, expectedStrings := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		for _, expected := range expectedStrings {
			if !strings.Contains(text, expected) {
				t.Fatalf("%s missing %q\n%s", path, expected, text)
			}
		}
		if strings.Contains(text, "Code Switch X X") {
			t.Fatalf("%s contains duplicated X marker\n%s", path, text)
		}
	}

	if _, err := os.Stat(filepath.Join("..", "build", "linux", "CodeSwitch.desktop")); !os.IsNotExist(err) {
		t.Fatalf("legacy Linux desktop file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestMainRuntimeUsesXApplicationIdentity(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "main.go"))
	if err != nil {
		t.Fatalf("ReadFile(main.go) error = %v", err)
	}
	source := string(content)
	if strings.Contains(source, `"Code Switch"`) {
		t.Fatalf("main.go still contains legacy runtime display name literal")
	}
	if !strings.Contains(source, "services.AppDisplayName") {
		t.Fatalf("main.go should use shared application identity constants")
	}
}
