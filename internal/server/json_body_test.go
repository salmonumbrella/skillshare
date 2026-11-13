package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeTestBody(body string, limit int64) (*httptest.ResponseRecorder, map[string]string, error) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	var dst map[string]string
	err := decodeJSON(rr, req, &dst, limit)
	return rr, dst, err
}

func TestDecodeJSON_Valid(t *testing.T) {
	_, dst, err := decodeTestBody(`{"name":"demo"}`, defaultJSONBodyLimit)
	if err != nil || dst["name"] != "demo" {
		t.Fatalf("got %v, err %v; want name=demo", dst, err)
	}
}

func TestDecodeJSON_MalformedLeavesResponseToCaller(t *testing.T) {
	rr, _, err := decodeTestBody(`{"name":`, defaultJSONBodyLimit)
	if err == nil || errors.Is(err, errBodyTooLarge) || rr.Body.Len() != 0 {
		t.Fatalf("want an unanswered decode error, got err %v, body %q", err, rr.Body.String())
	}
}

func TestDecodeJSON_OversizedAnswers413(t *testing.T) {
	rr, _, err := decodeTestBody(`{"name":"`+strings.Repeat("x", 64)+`"}`, 16)
	if !errors.Is(err, errBodyTooLarge) || rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got err %v, status %d; want errBodyTooLarge and 413", err, rr.Code)
	}
}

func TestHandleSaveConfig_OversizedBodyReturns413(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"raw":"` + strings.Repeat("x", int(defaultJSONBodyLimit)) + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rr.Body.String(), "byte limit") {
		t.Fatalf("expected 413 with a limit message, got %d: %s", rr.Code, rr.Body.String())
	}
}
