package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestPingerSuccess(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == testPingCommand {
			return "200 0        PONG 1.0      "
		}

		return ""
	})
	defer server.cleanup()

	msg := Pinger(context.Background(), server.address, testSecret)
	if !strings.Contains(msg, "PONG") {
		t.Fatalf("expected PONG in response, got %q", msg)
	}
}

func TestGetPingNotFound(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/service/missing/ping", http.NoBody)
	responseRecorder := httptest.NewRecorder()
	ps := httprouter.Params{{Key: testServiceKey, Value: testMissing}}

	appState.GetPing(responseRecorder, req, ps)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", responseRecorder.Code)
	}
}

func TestGetPingSuccess(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == testPingCommand {
			return "200 0        PONG 1.0      "
		}

		return ""
	})
	defer server.cleanup()

	appState := newTestApplication(Services{
		testGroup1: {Hosts: []string{server.address}, Secret: testSecret},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/service/group1/ping", http.NoBody)
	responseRecorder := httptest.NewRecorder()
	ps := httprouter.Params{{Key: testServiceKey, Value: testGroup1}}

	appState.GetPing(responseRecorder, req, ps)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseRecorder.Code)
	}

	var messages Messages

	err := json.Unmarshal(responseRecorder.Body.Bytes(), &messages)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if _, ok := messages[server.address]; !ok {
		t.Fatalf("expected message for host %s", server.address)
	}
}

func TestPingerDialError(t *testing.T) {
	t.Parallel()

	msg := Pinger(context.Background(), testClosedAddr, testSecret)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestPingerAuthError(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServerNoChallenge(t)
	defer server.cleanup()

	msg := Pinger(context.Background(), server.address, testSecret)
	if !strings.Contains(msg, "no challenge code") {
		t.Fatalf("expected auth error, got %q", msg)
	}
}

func TestPingerReadError(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == testPingCommand {
			return testCloseToken
		}

		return ""
	})
	defer server.cleanup()

	msg := Pinger(context.Background(), server.address, testSecret)
	if msg == "" || strings.Contains(msg, "PONG") {
		t.Fatalf("expected read error, got %q", msg)
	}
}
