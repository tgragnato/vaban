package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestGetServiceFound(t *testing.T) {
	withServices(t, Services{
		"group1": {Hosts: []string{"a:6082", "b:6082"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/service/group1", nil)
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "group1"}}

	GetService(rr, req, ps)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var hosts []string
	if err := json.Unmarshal(rr.Body.Bytes(), &hosts); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetServiceNotFound(t *testing.T) {
	withServices(t, Services{})

	req := httptest.NewRequest(http.MethodGet, "/v1/service/missing", nil)
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "missing"}}

	GetService(rr, req, ps)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestGetServices(t *testing.T) {
	withServices(t, Services{
		"group1": {Hosts: []string{"a:6082"}},
		"group2": {Hosts: []string{"b:6082"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	rr := httptest.NewRecorder()

	GetServices(rr, req, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var groups []string
	if err := json.Unmarshal(rr.Body.Bytes(), &groups); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}
