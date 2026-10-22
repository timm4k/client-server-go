package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"deviceregistry/internal/domain"
	"deviceregistry/internal/handler"
	"deviceregistry/internal/repository"
)

const validDevice = `{"name":"Thermal Sensor","ip_address":"192.168.1.50","type":"sensor","status":"online"}`

func request(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	return w
}

func TestDeviceLifecycle(t *testing.T) {
	var logs bytes.Buffer
	router := handler.NewRouter(repository.NewMemory(), log.New(&logs, "", 0))
	path := "/api/v1/devices"
	if response := request(router, "GET", path, ""); response.Code != 200 || strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s", response.Code, response.Body)
	}
	response := request(router, "POST", path, validDevice)
	if response.Code != 201 || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("create: %d %s", response.Code, response.Body)
	}
	var created domain.Device
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.ID == "" || created.UpdatedAt.IsZero() {
		t.Fatalf("invalid generated fields: %+v error=%v", created, err)
	}
	resource := path + "/" + created.ID
	if response.Header().Get("Location") != resource {
		t.Fatal("incorrect Location header")
	}
	for _, target := range []string{path, resource, resource + "/"} {
		if got := request(router, "GET", target, ""); got.Code != 200 || !strings.Contains(got.Body.String(), created.ID) {
			t.Fatalf("read %s: %d %s", target, got.Code, got.Body)
		}
	}
	updatedBody := `{"name":"Backup Gateway","ip_address":"::1","type":"gateway"}`
	response = request(router, "PUT", resource, updatedBody)
	var updated domain.Device
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil || response.Code != 200 || updated.ID != created.ID || updated.Name != "Backup Gateway" || updated.Status != "offline" || updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatalf("update: %d %+v error=%v", response.Code, updated, err)
	}
	if response := request(router, "PUT", resource, updatedBody); response.Code != 200 {
		t.Fatal("repeated PUT failed")
	}
	response = request(router, "DELETE", resource, "")
	if response.Code != 204 || response.Body.Len() != 0 {
		t.Fatalf("delete must have no body: %d %q", response.Code, response.Body)
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		if response := request(router, method, resource, validDevice); response.Code != 404 {
			t.Fatalf("%s after delete: %d", method, response.Code)
		}
	}
	for _, field := range []string{"method=POST", `path="/api/v1/devices"`, "status=201", "duration=", "remote="} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("missing log field %s", field)
		}
	}
}

func TestInvalidRequests(t *testing.T) {
	store := repository.NewMemory()
	router := handler.NewRouter(store, log.New(io.Discard, "", 0))
	device := domain.Device{ID: "existing", Name: "Original"}
	if err := store.Create(device); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"malformed":            `{"name":`,
		"missing-fields":       `{"name":"Sensor"}`,
		"blank-name":           `{"name":"  ","ip_address":"127.0.0.1","type":"sensor"}`,
		"invalid-ip":           `{"name":"Sensor","ip_address":"bad-ip","type":"sensor"}`,
		"invalid-status":       `{"name":"Sensor","ip_address":"127.0.0.1","type":"sensor","status":"unknown"}`,
		"extra-json":           validDevice + `{}`,
		"client-controlled-id": `{"id":"custom","name":"Sensor","ip_address":"127.0.0.1","type":"sensor"}`,
		"oversized":            `{"name":"` + strings.Repeat("x", 1<<20) + `"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			for _, method := range []string{"POST", "PUT"} {
				path := "/api/v1/devices"
				if method == "PUT" {
					path += "/existing"
				}
				if response := request(router, method, path, body); response.Code != 400 {
					t.Fatalf("%s returned %d: %s", method, response.Code, response.Body)
				}
			}
		})
	}
	if got, _ := store.GetByID(device.ID); got != device || len(store.GetAll()) != 1 {
		t.Fatal("invalid request modified repository")
	}
}

func TestConcurrentHTTPRequests(t *testing.T) {
	store := repository.NewMemory()
	server := httptest.NewServer(handler.NewRouter(store, log.New(io.Discard, "", 0)))
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.Post(server.URL+"/api/v1/devices", "application/json", strings.NewReader(validDevice))
			if err != nil {
				t.Error(err)
				return
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusCreated {
				t.Errorf("create status: %d", response.StatusCode)
			}
			list, err := client.Get(server.URL + "/api/v1/devices")
			if err != nil {
				t.Error(err)
				return
			}
			defer list.Body.Close()
			if list.StatusCode != 200 {
				t.Errorf("list status: %d", list.StatusCode)
			}
		}()
	}
	wg.Wait()
	if count := len(store.GetAll()); count != 24 {
		t.Fatalf("expected 24 unique devices, got %d", count)
	}
}
