package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthWithoutStore(t *testing.T) {
	server := NewServer(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["success"] != true {
		t.Fatalf("success = %v", body["success"])
	}
	if body["database"] != "disabled" {
		t.Fatalf("database = %v", body["database"])
	}
}

func TestNormalizeSlugsDeduplicates(t *testing.T) {
	got := normalizeSlugs([]string{"two-sum", " ", "Two-Sum", "add-two-numbers"})
	want := []string{"two-sum", "add-two-numbers"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
