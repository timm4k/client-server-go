package demo

import (
	"net/http"
	"net/http/httptest"
	"time"
)

func Targets() ([]string, func()) {
	mux := http.NewServeMux()
	for _, endpoint := range []struct {
		path   string
		status int
		delay  time.Duration
	}{
		{"/api", http.StatusOK, 300 * time.Millisecond},
		{"/shop", http.StatusOK, 250 * time.Millisecond},
		{"/missing", http.StatusNotFound, 150 * time.Millisecond},
		{"/maintenance", http.StatusServiceUnavailable, 200 * time.Millisecond},
	} {
		status, delay := endpoint.status, endpoint.delay
		mux.HandleFunc(endpoint.path, func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(delay):
				w.WriteHeader(status)
			case <-r.Context().Done():
			}
		})
	}
	server := httptest.NewServer(mux)
	return []string{
		server.URL + "/api",
		server.URL + "/shop",
		server.URL + "/missing",
		server.URL + "/maintenance",
		"://invalid-url",
	}, server.Close
}
