package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"deviceregistry/internal/domain"
	"deviceregistry/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type deviceInput struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Type      string `json:"type"`
	Status    string `json:"status"`
}

type Devices struct {
	repository repository.DeviceRepository
}

func NewRouter(store repository.DeviceRepository, logger *log.Logger) http.Handler {
	h := &Devices{repository: store}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)
			logger.Printf("[HTTP] method=%s path=%q status=%d duration=%s remote=%q", r.Method, r.URL.Path, wrapped.Status(), time.Since(start), r.RemoteAddr)
		})
	})
	r.Route("/api/v1/devices", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.get)
			r.Put("/", h.update)
			r.Delete("/", h.delete)
		})
	})
	return r
}

func (h *Devices) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.repository.GetAll())
}

func (h *Devices) get(w http.ResponseWriter, r *http.Request) {
	device, exists := h.repository.GetByID(chi.URLParam(r, "id"))
	if !exists {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (h *Devices) create(w http.ResponseWriter, r *http.Request) {
	device, ok := readDevice(w, r)
	if !ok {
		return
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		http.Error(w, "Could not generate device ID", http.StatusInternalServerError)
		return
	}
	device.ID = "dev-" + hex.EncodeToString(id[:])
	if err := h.repository.Create(device); err != nil {
		http.Error(w, "Could not create device", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", "/api/v1/devices/"+device.ID)
	writeJSON(w, http.StatusCreated, device)
}

func (h *Devices) update(w http.ResponseWriter, r *http.Request) {
	device, ok := readDevice(w, r)
	if !ok {
		return
	}
	device.ID = chi.URLParam(r, "id")
	if !h.repository.Update(device.ID, device) {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (h *Devices) delete(w http.ResponseWriter, r *http.Request) {
	if !h.repository.Delete(chi.URLParam(r, "id")) {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readDevice(w http.ResponseWriter, r *http.Request) (domain.Device, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input deviceInput
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
		return domain.Device{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Request body must contain exactly one JSON object", http.StatusBadRequest)
		return domain.Device{}, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.IPAddress = strings.TrimSpace(input.IPAddress)
	input.Type = strings.TrimSpace(input.Type)
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" || input.IPAddress == "" || input.Type == "" {
		http.Error(w, "Required fields: name, ip_address, type", http.StatusBadRequest)
		return domain.Device{}, false
	}
	if _, err := netip.ParseAddr(input.IPAddress); err != nil {
		http.Error(w, "ip_address must be a valid IPv4 or IPv6 address", http.StatusBadRequest)
		return domain.Device{}, false
	}
	if input.Status == "" {
		input.Status = "offline"
	}
	if input.Status != "online" && input.Status != "offline" {
		http.Error(w, "status must be online or offline", http.StatusBadRequest)
		return domain.Device{}, false
	}
	return domain.Device{Name: input.Name, IPAddress: input.IPAddress, Type: input.Type, Status: input.Status, UpdatedAt: time.Now().UTC()}, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
