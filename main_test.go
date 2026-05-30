package main

import (
	"testing"

	"codeswitch/services"
	_ "modernc.org/sqlite"
)

func TestRelayAddrFromSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings services.AppSettings
		want     string
	}{
		{
			name:     "default port",
			settings: services.AppSettings{},
			want:     ":18101",
		},
		{
			name:     "custom port",
			settings: services.AppSettings{RelayPort: 18111},
			want:     ":18111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := relayAddrFromSettings(tt.settings); got != tt.want {
				t.Fatalf("relayAddrFromSettings() = %q, want %q", got, tt.want)
			}
		})
	}
}
