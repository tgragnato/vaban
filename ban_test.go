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

func TestPostBanValidationErrors(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{testGroup1: {Hosts: []string{testClosedAddr}, Secret: ""}})

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "invalid json",
			body:       "{",
			wantStatus: http.StatusInternalServerError,
			wantBody:   "unexpected EOF",
		},
		{
			name:       "missing pattern and vcl",
			body:       `{"Pattern":"","Vcl":""}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Pattern or VCL is required",
		},
		{
			name:       "pattern without slash",
			body:       `{"Pattern":"abc"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Pattern must start with a /",
		},
		{
			name:       "pattern and vcl together",
			body:       `{"Pattern":"/a","Vcl":"req.http.host == \"a\""}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Pattern or VCL is required, not both",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/v1/service/group1/ban",
				strings.NewReader(testCase.body),
			)
			responseRecorder := httptest.NewRecorder()
			ps := httprouter.Params{{Key: testServiceKey, Value: testGroup1}}

			appState.PostBan(responseRecorder, req, ps)

			if responseRecorder.Code != testCase.wantStatus {
				t.Fatalf("expected status %d, got %d", testCase.wantStatus, responseRecorder.Code)
			}

			if !strings.Contains(responseRecorder.Body.String(), testCase.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", testCase.wantBody, responseRecorder.Body.String())
			}
		})
	}
}

func TestPostBanServiceNotFound(t *testing.T) {
	t.Parallel()

	appState := newTestApplication(Services{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/missing/ban",
		strings.NewReader(`{"Pattern":"/"}`),
	)
	responseRecorder := httptest.NewRecorder()
	ps := httprouter.Params{{Key: testServiceKey, Value: testMissing}}

	appState.PostBan(responseRecorder, req, ps)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", responseRecorder.Code)
	}
}

func TestBannerSuccessWithPattern(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.url ~ /foo$" {
			return testOKResponse
		}

		return ""
	})
	defer server.cleanup()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/service/group1/ban", http.NoBody)

	msg := Banner(context.Background(), server.address, BanPost{Pattern: "/foo", Vcl: ""}, testSecret, req)
	if !strings.Contains(msg, "ban status 200 0") {
		t.Fatalf("unexpected banner reply: %q", msg)
	}
}

func TestPostBanSuccess(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.url ~ /$" {
			return testOKResponse
		}

		return ""
	})
	defer server.cleanup()

	appState := newTestApplication(Services{
		testGroup1: {Hosts: []string{server.address}, Secret: testSecret},
	})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/service/group1/ban",
		strings.NewReader(`{"Pattern":"/"}`),
	)
	responseRecorder := httptest.NewRecorder()
	ps := httprouter.Params{{Key: testServiceKey, Value: testGroup1}}

	appState.PostBan(responseRecorder, req, ps)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseRecorder.Code)
	}

	var messages Messages

	err := json.Unmarshal(responseRecorder.Body.Bytes(), &messages)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if got, ok := messages[server.address]; !ok || !strings.Contains(got.Msg, "ban status 200 0") {
		t.Fatalf("unexpected response payload: %#v", messages)
	}
}

func TestBannerSuccessWithVCL(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.http.host == \"example.com\"" {
			return testOKResponse
		}

		return ""
	})
	defer server.cleanup()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/service/group1/ban", http.NoBody)

	msg := Banner(
		context.Background(),
		server.address,
		BanPost{Pattern: "", Vcl: `req.http.host == "example.com"`},
		testSecret,
		req,
	)
	if !strings.Contains(msg, "ban status 200 0") {
		t.Fatalf("unexpected banner reply: %q", msg)
	}
}

func TestBannerDialError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/service/group1/ban", http.NoBody)

	msg := Banner(context.Background(), testClosedAddr, BanPost{Pattern: "/", Vcl: ""}, testSecret, req)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestBannerAuthError(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServerNoChallenge(t)
	defer server.cleanup()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/service/group1/ban", http.NoBody)

	msg := Banner(context.Background(), server.address, BanPost{Pattern: "/", Vcl: ""}, testSecret, req)
	if !strings.Contains(msg, "no challenge code") {
		t.Fatalf("expected auth error, got %q", msg)
	}
}

func TestBannerReadErrorAfterCommand(t *testing.T) {
	t.Parallel()

	server := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.url ~ /$" {
			return testCloseToken
		}

		return ""
	})
	defer server.cleanup()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/service/group1/ban", http.NoBody)

	msg := Banner(context.Background(), server.address, BanPost{Pattern: "/", Vcl: ""}, testSecret, req)
	if msg == "" || strings.Contains(msg, "ban status") {
		t.Fatalf("expected read error, got %q", msg)
	}
}
