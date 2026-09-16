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

func TestStatusHealthParsesBackendList(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == testBackendList {
			return "Backend name Admin Probe Health\nboot.be1 probe 4/4 healthy\n"
		}

		return ""
	})
	defer server.cleanup()

	backends := StatusHealth(context.Background(), server.address, testSecret, "")

	healthStatus, ok := backends["boot.be1"]
	if !ok {
		t.Fatalf("expected backend boot.be1, got %#v", backends)
	}

	if healthStatus.Admin != "probe" || healthStatus.Probe != "4/4" || healthStatus.Health != "healthy" {
		t.Fatalf("unexpected backend status: %#v", healthStatus)
	}
}

func TestPostHealthRequiresSetHealth(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{testGroup1: {Hosts: []string{testClosedAddr}, Secret: ""}})

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		strings.NewReader(`{"Set_health":""}`),
	)
	responseRecorder := httptest.NewRecorder()
	params := httprouter.Params{{Key: testServiceKey, Value: testGroup1}, {Key: testBackendKey, Value: testBackendName}}

	appState.PostHealth(responseRecorder, request, params)

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", responseRecorder.Code)
	}
}

func TestGetHealthSuccess(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == testBackendList {
			return "Backend name Admin Probe Health\nboot.be1 probe 4/4 healthy\n"
		}

		return ""
	})
	defer server.cleanup()

	appState := newTestApplication(Services{testGroup1: {Hosts: []string{server.address}, Secret: testSecret}})

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/service/group1/health",
		http.NoBody,
	)
	responseRecorder := httptest.NewRecorder()
	params := httprouter.Params{{Key: testServiceKey, Value: testGroup1}}

	appState.GetHealth(responseRecorder, request, params)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseRecorder.Code)
	}

	var servers Servers

	err := json.Unmarshal(responseRecorder.Body.Bytes(), &servers)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if _, ok := servers[server.address]["boot.be1"]; !ok {
		t.Fatalf("expected backend result for host %s", server.address)
	}
}

func TestStatusHealthSpecificBackend(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.list " + testBackendName {
			return "Backend name Admin Probe Health\nbe1 probe 3/4 healthy\n"
		}

		return ""
	})
	defer server.cleanup()

	backends := StatusHealth(context.Background(), server.address, testSecret, testBackendName)
	if _, ok := backends[testBackendName]; !ok {
		t.Fatalf("expected backend be1, got %#v", backends)
	}
}

func TestGetHealthServiceNotFound(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{})

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/v1/service/missing/health",
		http.NoBody,
	)
	responseRecorder := httptest.NewRecorder()
	params := httprouter.Params{{Key: testServiceKey, Value: testMissing}}

	appState.GetHealth(responseRecorder, request, params)

	if !strings.Contains(responseRecorder.Body.String(), "Service could not be found.") {
		t.Fatalf("unexpected body: %q", responseRecorder.Body.String())
	}
}

func TestUpdateHealthSuccess(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.set_health " + testBackendName + " " + testHealthSick {
			return testOKResponse
		}

		return ""
	})
	defer server.cleanup()

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		http.NoBody,
	)

	msg := UpdateHealth(
		context.Background(),
		server.address,
		testSecret,
		testBackendName,
		HealthPost{SetHealth: testHealthSick},
		request,
	)
	if !strings.Contains(msg, "updated with status 200 0") {
		t.Fatalf("unexpected update response: %q", msg)
	}
}

func TestPostHealthInvalidJSON(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{testGroup1: {Hosts: []string{testClosedAddr}, Secret: ""}})

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		strings.NewReader("{"),
	)
	responseRecorder := httptest.NewRecorder()
	params := httprouter.Params{{Key: testServiceKey, Value: testGroup1}, {Key: testBackendKey, Value: testBackendName}}

	appState.PostHealth(responseRecorder, request, params)

	if responseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", responseRecorder.Code)
	}
}

func TestPostHealthSuccess(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.set_health " + testBackendName + " healthy" {
			return testOKResponse
		}

		return ""
	})
	defer server.cleanup()

	appState := newTestApplication(Services{testGroup1: {Hosts: []string{server.address}, Secret: testSecret}})

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		strings.NewReader(`{"Set_health":"healthy"}`),
	)
	responseRecorder := httptest.NewRecorder()
	params := httprouter.Params{{Key: testServiceKey, Value: testGroup1}, {Key: testBackendKey, Value: testBackendName}}

	appState.PostHealth(responseRecorder, request, params)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseRecorder.Code)
	}

	var messages Messages

	err := json.Unmarshal(responseRecorder.Body.Bytes(), &messages)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if got, ok := messages[server.address]; !ok || !strings.Contains(got.Msg, "updated with status 200 0") {
		t.Fatalf("unexpected response payload: %#v", messages)
	}
}

func TestPostHealthServiceNotFound(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{})

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/missing/health/be1",
		strings.NewReader(`{"Set_health":"sick"}`),
	)
	responseRecorder := httptest.NewRecorder()
	params := httprouter.Params{{Key: testServiceKey, Value: testMissing}, {Key: testBackendKey, Value: testBackendName}}

	appState.PostHealth(responseRecorder, request, params)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", responseRecorder.Code)
	}
}

func TestUpdateHealthDialError(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		http.NoBody,
	)

	msg := UpdateHealth(
		context.Background(),
		testClosedAddr,
		testSecret,
		testBackendName,
		HealthPost{SetHealth: testHealthSick},
		request,
	)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestUpdateHealthAuthError(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServerNoChallenge(t)
	defer server.cleanup()

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		http.NoBody,
	)

	msg := UpdateHealth(
		context.Background(),
		server.address,
		testSecret,
		testBackendName,
		HealthPost{SetHealth: testHealthSick},
		request,
	)
	if !strings.Contains(msg, "Could not write packet") && !strings.Contains(msg, "no challenge code") && msg == "" {
		t.Fatalf("unexpected response: %q", msg)
	}
}

func TestUpdateHealthReadError(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "backend.set_health be1 auto" {
			return testCloseToken
		}

		return ""
	})
	defer server.cleanup()

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/health/be1",
		http.NoBody,
	)

	msg := UpdateHealth(context.Background(), server.address, testSecret, "be1", HealthPost{SetHealth: "auto"}, request)
	if msg == "" || strings.Contains(msg, "updated with status") {
		t.Fatalf("expected read error, got %q", msg)
	}
}

func TestStatusHealthDialError(t *testing.T) {
	t.Parallel()

	backends := StatusHealth(context.Background(), testClosedAddr, testSecret, "")
	if len(backends) != 0 {
		t.Fatalf("expected empty result, got %#v", backends)
	}
}

func TestStatusHealthReadError(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == testBackendList {
			return testCloseToken
		}

		return ""
	})
	defer server.cleanup()

	backends := StatusHealth(context.Background(), server.address, testSecret, "")
	if len(backends) != 0 {
		t.Fatalf("expected empty result, got %#v", backends)
	}
}
