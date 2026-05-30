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
	if strings.Contains(path, string(filepath.Separator)+legacyDataDirName()+string(filepath.Separator)) {
		t.Fatalf("provider path = %q, must not use legacy data directory", path)
	}
}

func TestAppSettingsServiceUsesXSettingsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	service := NewAppSettingsService(nil)

	if !strings.Contains(service.path, string(filepath.Separator)+".codex-switch-x"+string(filepath.Separator)) {
		t.Fatalf("app settings path = %q, want app-owned .codex-switch-x directory", service.path)
	}
	if strings.Contains(service.path, string(filepath.Separator)+legacyCodexSettingsDirName()+string(filepath.Separator)) {
		t.Fatalf("app settings path = %q, must not use legacy settings directory", service.path)
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
	if strings.HasSuffix(darwinPath, filepath.Join("Library", "LaunchAgents", legacyBundleIdentifier()+".plist")) {
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
	if strings.Contains(string(plist), "<string>"+legacyBundleIdentifier()+"</string>") {
		t.Fatalf("darwin plist content = %q, must not use legacy bundle identifier label", string(plist))
	}

	linuxPath := service.getLinuxDesktopPath()
	if !strings.HasSuffix(linuxPath, filepath.Join("autostart", "codeswitch-x.desktop")) {
		t.Fatalf("linux desktop path = %q, want codeswitch-x.desktop", linuxPath)
	}
	if strings.HasSuffix(linuxPath, filepath.Join("autostart", legacyCompactName()+".desktop")) {
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

	if _, err := os.Stat(filepath.Join("..", "build", "linux", legacyCamelName()+".desktop")); !os.IsNotExist(err) {
		t.Fatalf("legacy Linux desktop file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestReleaseSurfaceUsesXApplicationIdentity(t *testing.T) {
	files := map[string]struct {
		required  []string
		forbidden []string
	}{
		filepath.Join("..", "scripts", "publish_release.sh"): {
			required: []string{
				"bin/CodeSwitchX.app",
				"CodeSwitchX.app",
				"codeswitch-x-macos-${arch}.zip",
				"codeswitch-x-linux-amd64.deb",
				"CodeSwitchX-amd64-installer.exe",
				"CodeSwitchX.exe",
			},
			forbidden: []string{
				"bin/" + legacyCamelName() + ".app",
				legacyCompactName() + "-macos-${arch}.zip",
				legacyCompactName() + "-linux-amd64.deb",
				legacyCompactName() + "-amd64-installer.exe",
				legacyCompactName() + ".exe",
			},
		},
		filepath.Join("..", ".github", "workflows", "release.yml"): {
			required: []string{
				"bin/CodeSwitchX.app",
				"codeswitch-x-macos-${{ matrix.arch }}.zip",
				"bin\\CodeSwitchX.exe",
				"CodeSwitchX-amd64-installer.exe",
				"codeswitch-x-linux-amd64.deb",
			},
			forbidden: []string{
				"bin/" + legacyCamelName() + ".app",
				legacyCompactName() + "-macos-${{ matrix.arch }}.zip",
				"bin\\" + legacyCamelName() + ".exe",
				legacyCamelName() + "-amd64-installer.exe",
				legacyCompactName() + "-linux-amd64.deb",
			},
		},
		filepath.Join("..", "README.md"): {
			required: []string{
				"# Code Switch X",
				"clone 自 https://github.com/" + legacyRepoOwner() + "/" + legacyDashedName(),
				"https://github.com/mysekai7/code-switch-x/releases",
				"resources/images/code-switch-x.png",
				"codeswitch-x-macos-arm64.zip",
				"CodeSwitchX.exe",
			},
			forbidden: []string{
				"# " + legacyDisplayName() + "\n",
				"https://github.com/" + legacyRepoOwner() + "/code-switch-x/releases",
				"https://github.com/" + legacyRepoOwner() + "/" + legacyDashedName() + "/releases",
				"https://github.com/" + legacyRepoOwner() + "/" + legacyTypoDashedName() + "/releases",
				"resources/images/" + legacyDashedName() + ".png",
				legacyCompactName() + "-macos-arm64.zip",
				legacyCompactName() + ".exe",
			},
		},
		filepath.Join("..", "RELEASE_NOTES.md"): {
			required:  []string{"# Code Switch X v0.1.8"},
			forbidden: []string{"# " + legacyDisplayName() + " v0.1.8"},
		},
		filepath.Join("..", "requestlog_mock_test.go"): {
			required:  []string{`".code-switch-x"`},
			forbidden: []string{`"` + legacyDataDirName() + `"`},
		},
		filepath.Join("..", "frontend", "src", "components", "Main", "Index.vue"): {
			required: []string{
				"https://github.com/mysekai7/code-switch-x/releases",
				"https://api.github.com/repos/mysekai7/code-switch-x/releases/latest",
			},
			forbidden: []string{
				"https://github.com/" + legacyRepoOwner() + "/code-switch-x/releases",
				"https://api.github.com/repos/" + legacyRepoOwner() + "/code-switch-x/releases/latest",
				"https://github.com/" + legacyRepoOwner() + "/" + legacyDashedName() + "/releases",
				"https://api.github.com/repos/" + legacyRepoOwner() + "/" + legacyDashedName() + "/releases/latest",
			},
		},
	}

	for path, checks := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		text := string(content)
		for _, required := range checks.required {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q\n%s", path, required, text)
			}
		}
		for _, forbidden := range checks.forbidden {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains legacy application identity %q\n%s", path, forbidden, text)
			}
		}
	}

	expectedImages := []string{
		filepath.Join("..", "resources", "images", "code-switch-x.png"),
		filepath.Join("..", "resources", "images", "code-switch-x-logs.png"),
		filepath.Join("..", "resources", "images", "code-switch-x-logs-dark.png"),
		filepath.Join("..", "resources", "images", "code-switch-x-dark.png"),
	}
	for _, path := range expectedImages {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected X image asset %q: %v", path, err)
		}
	}

	legacyImages := []string{
		filepath.Join("..", "resources", "images", legacyDashedName()+".png"),
		filepath.Join("..", "resources", "images", legacyDashedName()+"-logs.png"),
		filepath.Join("..", "resources", "images", legacyDashedName()+"-logs-dark.png"),
		filepath.Join("..", "resources", "images", legacyTypoDashedName()+"-dark.png"),
	}
	for _, path := range legacyImages {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("legacy image asset %q still exists or stat failed unexpectedly: %v", path, err)
		}
	}
}

func TestMainRuntimeUsesXApplicationIdentity(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "main.go"))
	if err != nil {
		t.Fatalf("ReadFile(main.go) error = %v", err)
	}
	source := string(content)
	if strings.Contains(source, `"`+legacyDisplayName()+`"`) {
		t.Fatalf("main.go still contains legacy runtime display name literal")
	}
	if !strings.Contains(source, "services.AppDisplayName") {
		t.Fatalf("main.go should use shared application identity constants")
	}
}

func legacyDashedName() string {
	return "code" + "-switch"
}

func legacyTypoDashedName() string {
	return "code" + "-swtich"
}

func legacyCompactName() string {
	return "code" + "switch"
}

func legacyCamelName() string {
	return "Code" + "Switch"
}

func legacyDisplayName() string {
	return "Code" + " Switch"
}

func legacyBundleIdentifier() string {
	return "com." + legacyCompactName() + ".app"
}

func legacyDataDirName() string {
	return "." + legacyDashedName()
}

func legacyCodexSettingsDirName() string {
	return ".codex" + "-switch"
}

func legacyRepoOwner() string {
	return "dao" + "dao97"
}
