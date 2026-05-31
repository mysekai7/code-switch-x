package services

import (
	"net/http"
	"testing"
)

func TestCopyHTTPHeaderStripsHopByHopHeaders(t *testing.T) {
	dst := http.Header{}
	src := http.Header{
		"Content-Type":       {"application/json"},
		"Connection":         {"keep-alive, X-Internal-Hop"},
		"Keep-Alive":         {"timeout=5"},
		"Transfer-Encoding":  {"chunked"},
		"X-Internal-Hop":     {"remove-me"},
		"X-Upstream-Request": {"keep-me"},
	}

	copyHTTPHeader(dst, src)

	if got := dst.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := dst.Get("X-Upstream-Request"); got != "keep-me" {
		t.Fatalf("X-Upstream-Request = %q, want keep-me", got)
	}
	for _, header := range []string{"Connection", "Keep-Alive", "Transfer-Encoding", "X-Internal-Hop"} {
		if got := dst.Get(header); got != "" {
			t.Fatalf("%s = %q, want stripped", header, got)
		}
	}
}
