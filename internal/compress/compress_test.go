package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareCompressesResponseWithHandlerContentLength(t *testing.T) {
	body := strings.Repeat("module export const page = 'workspace';\n", 100)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Content-Length", "999999")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	Middleware(handler).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if got := response.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := response.Header.Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length = %q, want removed for compressed response", got)
	}

	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("open gzip response: %v", err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	if !bytes.Equal(decoded, []byte(body)) {
		t.Fatalf("decoded body differs from original")
	}
}

func TestMiddlewarePreservesContentLengthForSmallResponse(t *testing.T) {
	body := "small response"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "14")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	Middleware(handler).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if got := response.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want no compression", got)
	}
	if got := response.Header.Get("Content-Length"); got != "14" {
		t.Fatalf("Content-Length = %q, want 14", got)
	}
	decoded, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != body {
		t.Fatalf("body = %q, want %q", decoded, body)
	}
}
