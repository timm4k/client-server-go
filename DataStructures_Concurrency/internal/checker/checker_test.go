package checker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPChecker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	checker := NewHTTPChecker(200 * time.Millisecond)
	defer checker.Client.CloseIdleConnections()
	for _, tc := range []struct {
		name   string
		url    string
		status int
		failed bool
	}{
		{"ok", server.URL, 200, false},
		{"http404", server.URL + "/missing", 404, false},
		{"invalid", "://invalid-url", 0, true},
		{"timeout", server.URL + "/slow", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := checker.Check(tc.url)
			if result.URL != tc.url || result.StatusCode != tc.status || (result.Err != nil) != tc.failed {
				t.Fatalf("unexpected result: %+v", result)
			}
			if tc.name == "timeout" && result.Duration < checker.Client.Timeout {
				t.Fatal("timeout duration was not recorded correctly")
			}
		})
	}
}
