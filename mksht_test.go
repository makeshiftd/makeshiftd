package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestIndexHandler(t *testing.T) {
	configPath := filepath.Join("test", "config", "makeshiftd.json")
	indexPath := filepath.Join("test", "docs", "root1", "index.html")

	config, cerr := LoadAppConfig(configPath)
	AssertNotError(t, cerr, "Error loading config")

	handler := NewMkShtHanlder(config)

	req := httptest.NewRequest("GET", "/root1/index.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res := rec.Result()

	AssertEqualInt(t, http.StatusOK, res.StatusCode, "GET /root1/index.html: Response status")
	AssertEqual(t, res.Header.Get("Content-Type"), "text/html; charset=utf-8", "GET /root1/index.html: Response type")
	AssertEqualBytes(t, MustReadFile(t, indexPath), MustReadAll(t, res.Body), "GET /root1/index.html: Response content")

	req = httptest.NewRequest("GET", "/root1", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res = rec.Result()

	AssertEqualInt(t, http.StatusOK, res.StatusCode, "GET /root1: Response status")
	AssertEqual(t, res.Header.Get("Content-Type"), "text/html; charset=utf-8", "GET /root1: Response type")
	AssertEqualBytes(t, MustReadFile(t, indexPath), MustReadAll(t, res.Body), "GET /root1: Response body")

	req = httptest.NewRequest("GET", "/root1/index.html/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res = rec.Result()

	AssertEqualInt(t, http.StatusPermanentRedirect, res.StatusCode, "GET /root1: Response status")
}
