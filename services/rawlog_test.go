package services

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRedactHeadersRedactsSensitiveNamesCaseInsensitively(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret")
	headers.Set("x-api-key", "secret-key")
	headers.Set("Cookie", "session=secret")
	headers.Set("Anthropic-Api-Key", "anthropic-secret")
	headers.Set("Content-Type", "application/json")

	redacted := redactHeaders(headers)

	for _, key := range []string{"Authorization", "X-Api-Key", "Cookie", "Anthropic-Api-Key"} {
		if got := redacted.Get(key); got != rawLogRedactedValue {
			t.Fatalf("%s = %q, want redacted value", key, got)
		}
	}
	if got := redacted.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestMarshalRedactedHeadersDoesNotLeakSensitiveValues(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret")
	headers.Set("Content-Type", "application/json")

	payload := marshalRedactedHeaders(headers)
	if strings.Contains(payload, "secret") {
		t.Fatalf("marshaled headers leaked sensitive value: %s", payload)
	}

	var decoded map[string][]string
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("headers payload is not JSON: %v", err)
	}
	if decoded["Authorization"][0] != rawLogRedactedValue {
		t.Fatalf("Authorization = %q, want redacted", decoded["Authorization"][0])
	}
}

func TestCaptureBoundedTruncatesAndMarksOverflow(t *testing.T) {
	body, truncated := captureBounded([]byte("abcdef"), 3)

	if body != "abc" {
		t.Fatalf("body = %q, want abc", body)
	}
	if !truncated {
		t.Fatal("truncated = false, want true")
	}
}

func TestCaptureBoundedUsesDefaultForInvalidLimit(t *testing.T) {
	body, truncated := captureBounded([]byte("small"), 0)

	if body != "small" {
		t.Fatalf("body = %q, want small", body)
	}
	if truncated {
		t.Fatal("truncated = true, want false")
	}
}
