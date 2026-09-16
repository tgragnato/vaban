package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestPingerSuccess(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ping" {
			return "200 0        PONG 1.0      "
		}
		return ""
	})
	defer cleanup()

	msg := Pinger(addr, "secret")
	if !strings.Contains(msg, "PONG") {
		t.Fatalf("expected PONG in response, got %q", msg)
	}
}

func TestGetPingNotFound(t *testing.T) {
	withServices(t, Services{})

	req := httptest.NewRequest(http.MethodGet, "/v1/service/missing/ping", nil)
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "missing"}}

	GetPing(rr, req, ps)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestGetPingSuccess(t *testing.T) {
	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ping" {
			return "200 0        PONG 1.0      "
		}
		return ""
	})
	defer cleanup()

	withServices(t, Services{
		"group1": {Hosts: []string{addr}, Secret: "secret"},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/service/group1/ping", nil)
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "group1"}}

	GetPing(rr, req, ps)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var messages Messages
	if err := json.Unmarshal(rr.Body.Bytes(), &messages); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := messages[addr]; !ok {
		t.Fatalf("expected message for host %s", addr)
	}
}

func TestPingerDialError(t *testing.T) {
	t.Parallel()

	msg := Pinger("127.0.0.1:1", "secret")
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestPingerAuthError(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServerNoChallenge(t)
	defer cleanup()

	msg := Pinger(addr, "secret")
	if !strings.Contains(msg, "no challenge code") {
		t.Fatalf("expected auth error, got %q", msg)
	}
}

func TestPingerReadError(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ping" {
			return "__CLOSE__"
		}
		return ""
	})
	defer cleanup()

	msg := Pinger(addr, "secret")
	if msg == "" || strings.Contains(msg, "PONG") {
		t.Fatalf("expected read error, got %q", msg)
	}
}
