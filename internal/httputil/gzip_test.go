package httputil

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGzip_Compressed(t *testing.T) {
	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip encoding")
	}

	// Verify it's valid gzip
	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("invalid gzip: %v", err)
	}
	body, _ := io.ReadAll(reader)
	if string(body) != "hello world" {
		t.Errorf("body = %s", body)
	}
}

func TestGzip_NoAcceptEncoding(t *testing.T) {
	handler := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not compress without Accept-Encoding")
	}
	if w.Body.String() != "hello" {
		t.Errorf("body = %s", w.Body.String())
	}
}
