package services

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var upstreamHTTPClient = &http.Client{
	Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
}

func postUpstream(targetURL string, query map[string]string, headers map[string]string, body []byte) (*http.Response, error) {
	requestURL, err := addQueryParams(targetURL, query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return upstreamHTTPClient.Do(req)
}

func addQueryParams(targetURL string, query map[string]string) (string, error) {
	if len(query) == 0 {
		return targetURL, nil
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	for key, value := range query {
		values.Add(key, value)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func writeUpstreamResponse(w http.ResponseWriter, resp *http.Response, hooks ...func([]byte) (bool, []byte)) (int64, error) {
	if resp == nil {
		return 0, fmt.Errorf("raw response is nil")
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	statusCode := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")
	writeHeaders := func() {
		copyHTTPHeader(w.Header(), resp.Header)
		w.WriteHeader(statusCode)
	}

	if statusCode >= http.StatusBadRequest {
		writeHeaders()
		return writeNonStreamResponse(w, resp.Body, hooks...)
	}

	if strings.Contains(contentType, "text/event-stream") && resp.Body != nil {
		return writeStreamResponse(w, resp, writeHeaders, hooks...)
	}

	writeHeaders()
	return writeNonStreamResponse(w, resp.Body, hooks...)
}

func writeStreamResponse(
	w http.ResponseWriter,
	resp *http.Response,
	writeHeaders func(),
	hooks ...func([]byte) (bool, []byte),
) (int64, error) {
	reader := bufio.NewReader(resp.Body)

	firstLine, err := reader.Peek(1024)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("error peeking response: %w", err)
	}

	if !bytes.Contains(firstLine, []byte("\n")) && err == io.EOF {
		resp.Header.Set("Content-Type", "application/json")
		writeHeaders()

		allData, readErr := io.ReadAll(reader)
		if readErr != nil {
			return 0, fmt.Errorf("error reading non-standard response: %w", readErr)
		}
		return writeHookedChunk(w, allData, hooks...)
	}

	writeHeaders()
	totalBytes := int64(0)
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil {
			if readErr != io.EOF {
				return totalBytes, fmt.Errorf("error streaming response: %w", readErr)
			}
			if len(line) > 0 {
				n, writeErr := writeHookedChunk(w, line, hooks...)
				totalBytes += n
				if writeErr != nil {
					return totalBytes, fmt.Errorf("error writing final line: %w", writeErr)
				}
			}
			return totalBytes, nil
		}

		originalLine := append([]byte(nil), line...)
		flush := true
		processedLine := originalLine
		for _, hook := range hooks {
			flush, processedLine = hook(processedLine)
		}
		if !flush {
			continue
		}
		n, writeErr := w.Write(processedLine)
		totalBytes += int64(n)
		if writeErr != nil {
			return totalBytes, fmt.Errorf("error writing response: %w", writeErr)
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func writeNonStreamResponse(w http.ResponseWriter, body io.Reader, hooks ...func([]byte) (bool, []byte)) (int64, error) {
	if body == nil {
		return 0, nil
	}
	if len(hooks) == 0 {
		return io.Copy(w, body)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	return writeHookedChunk(w, data, hooks...)
}

func writeHookedChunk(w http.ResponseWriter, data []byte, hooks ...func([]byte) (bool, []byte)) (int64, error) {
	flush := true
	processedData := data
	for _, hook := range hooks {
		flush, processedData = hook(processedData)
	}
	if !flush || len(processedData) == 0 {
		return 0, nil
	}
	n, err := w.Write(processedData)
	return int64(n), err
}

func copyHTTPHeader(dst http.Header, src http.Header) {
	if dst == nil || src == nil {
		return
	}
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
