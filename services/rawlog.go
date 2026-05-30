package services

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/daodao97/xgo/xdb"
)

const rawLogRedactedValue = "[REDACTED]"

var sensitiveRawLogHeaders = map[string]bool{
	"authorization":     true,
	"cookie":            true,
	"set-cookie":        true,
	"x-api-key":         true,
	"api-key":           true,
	"anthropic-api-key": true,
}

func redactHeaders(headers http.Header) http.Header {
	redacted := make(http.Header, len(headers))
	for key, values := range headers {
		copied := append([]string(nil), values...)
		if sensitiveRawLogHeaders[strings.ToLower(key)] {
			copied = []string{rawLogRedactedValue}
		}
		redacted[key] = copied
	}
	return redacted
}

func marshalRedactedHeaders(headers http.Header) string {
	payload, err := json.Marshal(redactHeaders(headers))
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func captureBounded(data []byte, maxBytes int) (string, bool) {
	maxBytes = normalizeRawLogMaxBytes(maxBytes)
	if len(data) <= maxBytes {
		return string(data), false
	}
	return string(data[:maxBytes]), true
}

func normalizeRawLogMaxBytes(maxBytes int) int {
	if maxBytes <= 0 {
		return defaultRawLogMaxBytes
	}
	return maxBytes
}

type boundedRawLogBuffer struct {
	maxBytes  int
	data      []byte
	truncated bool
}

func newBoundedRawLogBuffer(maxBytes int) boundedRawLogBuffer {
	return boundedRawLogBuffer{maxBytes: normalizeRawLogMaxBytes(maxBytes)}
}

func (b *boundedRawLogBuffer) append(data []byte) {
	if len(data) == 0 {
		return
	}
	maxBytes := normalizeRawLogMaxBytes(b.maxBytes)
	if len(b.data) >= maxBytes {
		b.truncated = true
		return
	}
	remaining := maxBytes - len(b.data)
	if len(data) > remaining {
		b.data = append(b.data, data[:remaining]...)
		b.truncated = true
		return
	}
	b.data = append(b.data, data...)
}

func (b *boundedRawLogBuffer) set(data []byte) {
	body, truncated := captureBounded(data, b.maxBytes)
	b.data = []byte(body)
	b.truncated = truncated
}

func (b *boundedRawLogBuffer) string() string {
	return string(b.data)
}

type rawLogCapture struct {
	maxBytes             int
	requestHeaders       string
	requestBody          string
	responseHeaders      string
	responseBody         boundedRawLogBuffer
	upstreamRequestBody  string
	upstreamResponseBody string
	requestTruncated     bool
	responseTruncated    bool
}

func newRawLogCapture(settings AppSettings, requestHeaders http.Header, requestBody []byte) *rawLogCapture {
	if !settings.CaptureRawLogs {
		return nil
	}
	maxBytes := normalizeRawLogMaxBytes(settings.RawLogMaxBytes)
	body, truncated := captureBounded(requestBody, maxBytes)
	return &rawLogCapture{
		maxBytes:         maxBytes,
		requestHeaders:   marshalRedactedHeaders(requestHeaders),
		requestBody:      body,
		requestTruncated: truncated,
		responseBody:     newBoundedRawLogBuffer(maxBytes),
	}
}

func (c *rawLogCapture) captureResponseHeaders(headers http.Header) {
	if c == nil {
		return
	}
	c.responseHeaders = marshalRedactedHeaders(headers)
}

func (c *rawLogCapture) captureResponseBody(data []byte) {
	if c == nil {
		return
	}
	c.responseBody.set(data)
	c.responseTruncated = c.responseBody.truncated
}

func (c *rawLogCapture) captureUpstreamRequestBody(data []byte) {
	if c == nil {
		return
	}
	body, truncated := captureBounded(data, c.maxBytes)
	c.upstreamRequestBody = body
	if truncated {
		c.requestTruncated = true
	}
}

func (c *rawLogCapture) captureUpstreamResponseBody(data []byte) {
	if c == nil {
		return
	}
	body, truncated := captureBounded(data, c.maxBytes)
	c.upstreamResponseBody = body
	if truncated {
		c.responseTruncated = true
	}
}

func (c *rawLogCapture) responseHook() func([]byte) (bool, []byte) {
	return func(data []byte) (bool, []byte) {
		if c != nil {
			c.responseBody.append(data)
			c.responseTruncated = c.responseBody.truncated
		}
		return true, data
	}
}

func (c *rawLogCapture) insert(logID int64) error {
	if c == nil || logID == 0 {
		return nil
	}
	_, err := xdb.New("request_log_payload").Insert(xdb.Record{
		"log_id":                 logID,
		"request_headers":        c.requestHeaders,
		"request_body":           c.requestBody,
		"response_headers":       c.responseHeaders,
		"response_body":          c.responseBody.string(),
		"upstream_request_body":  c.upstreamRequestBody,
		"upstream_response_body": c.upstreamResponseBody,
		"request_truncated":      boolToInt(c.requestTruncated),
		"response_truncated":     boolToInt(c.responseTruncated),
	})
	return err
}
