package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestStatusHealthParsesBackendList(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.list" {
			return "Backend name Admin Probe Health\nboot.be1 probe 4/4 healthy\n"
		}
		return ""
	})
	defer cleanup()

	backends := StatusHealth(addr, "secret", "")

	hs, ok := backends["boot.be1"]
	if !ok {
		t.Fatalf("expected backend boot.be1, got %#v", backends)
	}
	if hs.Admin != "probe" || hs.Probe != "4/4" || hs.Health != "healthy" {
		t.Fatalf("unexpected backend status: %#v", hs)
	}
}

func TestPostHealthRequiresSetHealth(t *testing.T) {
	withServices(t, Services{"group1": {Hosts: []string{"127.0.0.1:1"}}})

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", strings.NewReader(`{"Set_health":""}`))
	rr := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "service", Value: "group1"},
		{Key: "backend", Value: "be1"},
	}

	PostHealth(rr, req, ps)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestGetHealthSuccess(t *testing.T) {
	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.list" {
			return "Backend name Admin Probe Health\nboot.be1 probe 4/4 healthy\n"
		}
		return ""
	})
	defer cleanup()

	withServices(t, Services{
		"group1": {Hosts: []string{addr}, Secret: "secret"},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/service/group1/health", nil)
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "group1"}}

	GetHealth(rr, req, ps)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var servers Servers
	if err := json.Unmarshal(rr.Body.Bytes(), &servers); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := servers[addr]["boot.be1"]; !ok {
		t.Fatalf("expected backend result for host %s", addr)
	}
}

func TestStatusHealthSpecificBackend(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.list be1" {
			return "Backend name Admin Probe Health\nbe1 probe 3/4 healthy\n"
		}
		return ""
	})
	defer cleanup()

	backends := StatusHealth(addr, "secret", "be1")
	if _, ok := backends["be1"]; !ok {
		t.Fatalf("expected backend be1, got %#v", backends)
	}
}

func TestGetHealthServiceNotFound(t *testing.T) {
	withServices(t, Services{})

	req := httptest.NewRequest(http.MethodGet, "/v1/service/missing/health", nil)
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "missing"}}

	GetHealth(rr, req, ps)

	if !strings.Contains(rr.Body.String(), "Service could not be found.") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestUpdateHealthSuccess(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.set_health be1 sick" {
			return "200 0       \n"
		}
		return ""
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", nil)
	msg := UpdateHealth(addr, "secret", "be1", HealthPost{Set_health: "sick"}, req)
	if !strings.Contains(msg, "updated with status 200 0") {
		t.Fatalf("unexpected update response: %q", msg)
	}
}

func TestPostHealthInvalidJSON(t *testing.T) {
	withServices(t, Services{"group1": {Hosts: []string{"127.0.0.1:1"}}})

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", strings.NewReader("{"))
	rr := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "service", Value: "group1"},
		{Key: "backend", Value: "be1"},
	}

	PostHealth(rr, req, ps)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestPostHealthSuccess(t *testing.T) {
	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.set_health be1 healthy" {
			return "200 0       \n"
		}
		return ""
	})
	defer cleanup()

	withServices(t, Services{
		"group1": {Hosts: []string{addr}, Secret: "secret"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", strings.NewReader(`{"Set_health":"healthy"}`))
	rr := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "service", Value: "group1"},
		{Key: "backend", Value: "be1"},
	}

	PostHealth(rr, req, ps)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var messages Messages
	if err := json.Unmarshal(rr.Body.Bytes(), &messages); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got, ok := messages[addr]; !ok || !strings.Contains(got.Msg, "updated with status 200 0") {
		t.Fatalf("unexpected response payload: %#v", messages)
	}
}

func TestPostHealthServiceNotFound(t *testing.T) {
	withServices(t, Services{})

	req := httptest.NewRequest(http.MethodPost, "/v1/service/missing/health/be1", strings.NewReader(`{"Set_health":"sick"}`))
	rr := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "service", Value: "missing"},
		{Key: "backend", Value: "be1"},
	}

	PostHealth(rr, req, ps)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestUpdateHealthDialError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", nil)
	msg := UpdateHealth("127.0.0.1:1", "secret", "be1", HealthPost{Set_health: "sick"}, req)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestUpdateHealthAuthError(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServerNoChallenge(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", nil)
	msg := UpdateHealth(addr, "secret", "be1", HealthPost{Set_health: "sick"}, req)
	if !strings.Contains(msg, "Could not write packet") && !strings.Contains(msg, "no challenge code") && msg == "" {
		t.Fatalf("unexpected response: %q", msg)
	}
}

func TestUpdateHealthReadError(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.set_health be1 auto" {
			return "__CLOSE__"
		}
		return ""
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/health/be1", nil)
	msg := UpdateHealth(addr, "secret", "be1", HealthPost{Set_health: "auto"}, req)
	if msg == "" || strings.Contains(msg, "updated with status") {
		t.Fatalf("expected read error, got %q", msg)
	}
}

func TestStatusHealthDialError(t *testing.T) {
	t.Parallel()

	backends := StatusHealth("127.0.0.1:1", "secret", "")
	if len(backends) != 0 {
		t.Fatalf("expected empty result, got %#v", backends)
	}
}

func TestStatusHealthReadError(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.list" {
			return "__CLOSE__"
		}
		return ""
	})
	defer cleanup()

	backends := StatusHealth(addr, "secret", "")
	if len(backends) != 0 {
		t.Fatalf("expected empty result, got %#v", backends)
	}
}
