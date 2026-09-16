package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLoggerDefaults(t *testing.T) {
	t.Parallel()

	m := NewLogger()
	if m == nil || m.Logger == nil {
		t.Fatal("expected logger middleware to be initialized")
	}
	if m.Name != "vaban" {
		t.Fatalf("unexpected logger name: %q", m.Name)
	}
}

func TestInitializeRootAndServicesRoutes(t *testing.T) {
	withServices(t, Services{
		"group1": {Hosts: []string{"a:6082"}},
	})

	n := initialize()

	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	n.ServeHTTP(rootRec, rootReq)
	if rootRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on root route, got %d", rootRec.Code)
	}

	svcReq := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	svcRec := httptest.NewRecorder()
	n.ServeHTTP(svcRec, svcReq)
	if svcRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on services route, got %d", svcRec.Code)
	}
}
