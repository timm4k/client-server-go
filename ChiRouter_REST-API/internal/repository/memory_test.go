package repository_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"deviceregistry/internal/domain"
	"deviceregistry/internal/repository"
)

func TestRepositoryLifecycle(t *testing.T) {
	r := repository.NewMemory()
	if devices := r.GetAll(); devices == nil || len(devices) != 0 {
		t.Fatalf("empty collection must be a non-nil empty slice: %v", devices)
	}
	device := domain.Device{ID: "dev-01", Name: "Sensor"}
	if err := r.Create(device); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(device); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate create: %v", err)
	}
	snapshot := r.GetAll()
	snapshot[0].Name = "Changed snapshot"
	if got, ok := r.GetByID(device.ID); !ok || got != device {
		t.Fatalf("stored device changed through snapshot: %+v", got)
	}
	device.Name = "Updated sensor"
	device.ID = "wrong-id"
	if !r.Update("dev-01", device) {
		t.Fatal("update failed")
	}
	if got, ok := r.GetByID("dev-01"); !ok || got.ID != "dev-01" || got.Name != device.Name {
		t.Fatalf("update did not preserve path ID: %+v", got)
	}
	if !r.Delete("dev-01") || r.Delete("dev-01") || r.Update("missing", device) {
		t.Fatal("incorrect missing-resource behavior")
	}
}

func TestConcurrentRepositoryAccess(t *testing.T) {
	r := repository.NewMemory()
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				id := fmt.Sprintf("dev-%02d-%02d", worker, i)
				device := domain.Device{ID: id, Name: "Sensor"}
				if err := r.Create(device); err != nil {
					t.Error(err)
				}
				r.GetAll()
				if _, ok := r.GetByID(id); !ok || !r.Update(id, device) || !r.Delete(id) {
					t.Errorf("lifecycle failed for %s", id)
				}
			}
		}(worker)
	}
	wg.Wait()
	if count := len(r.GetAll()); count != 0 {
		t.Fatalf("remaining devices: %d", count)
	}
}
