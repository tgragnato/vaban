package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testHostA = "a:6082"

func TestNewLoggerDefaults(t *testing.T) {
	t.Parallel()

	middleware := NewLogger()
	if middleware == nil || middleware.Logger == nil {
		t.Fatal("expected logger middleware to be initialized")
	}

	if middleware.Name != "vaban" {
		t.Fatalf("unexpected logger name: %q", middleware.Name)
	}
}

func TestInitializeRootAndServicesRoutes(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{
		testGroup1: {Hosts: []string{testHostA}, Secret: ""},
	})

	app := appState.initialize()

	rootReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	rootRec := httptest.NewRecorder()
	app.ServeHTTP(rootRec, rootReq)

	if rootRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on root route, got %d", rootRec.Code)
	}

	svcReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/services", http.NoBody)
	svcRec := httptest.NewRecorder()
	app.ServeHTTP(svcRec, svcReq)

	if svcRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on services route, got %d", svcRec.Code)
	}
}
