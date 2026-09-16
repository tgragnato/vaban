package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestGetServiceFound(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{
		testGroup1: {Hosts: []string{"a:6082", "b:6082"}, Secret: ""},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/service/group1", http.NoBody)
	responseRecorder := httptest.NewRecorder()
	ps := httprouter.Params{{Key: testServiceKey, Value: testGroup1}}

	appState.GetService(responseRecorder, req, ps)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseRecorder.Code)
	}

	var hosts []string

	err := json.Unmarshal(responseRecorder.Body.Bytes(), &hosts)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetServiceNotFound(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/service/missing", http.NoBody)
	responseRecorder := httptest.NewRecorder()
	ps := httprouter.Params{{Key: testServiceKey, Value: testMissing}}

	appState.GetService(responseRecorder, req, ps)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", responseRecorder.Code)
	}
}

func TestGetServices(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{
		testGroup1: {Hosts: []string{"a:6082"}, Secret: ""},
		"group2":   {Hosts: []string{"b:6082"}, Secret: ""},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/services", http.NoBody)
	responseRecorder := httptest.NewRecorder()

	appState.GetServices(responseRecorder, req, nil)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseRecorder.Code)
	}

	var groups []string

	err := json.Unmarshal(responseRecorder.Body.Bytes(), &groups)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}
