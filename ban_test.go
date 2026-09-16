package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func TestPostBanValidationErrors(t *testing.T) {
	withServices(t, Services{"group1": {Hosts: []string{"127.0.0.1:1"}}})

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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			ps := httprouter.Params{{Key: "service", Value: "group1"}}

			PostBan(rr, req, ps)

			if rr.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), tc.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", tc.wantBody, rr.Body.String())
			}
		})
	}
}

func TestPostBanServiceNotFound(t *testing.T) {
	withServices(t, Services{})

	req := httptest.NewRequest(http.MethodPost, "/v1/service/missing/ban", strings.NewReader(`{"Pattern":"/"}`))
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "missing"}}

	PostBan(rr, req, ps)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestBannerSuccessWithPattern(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.url ~ /foo$" {
			return "200 0       \n"
		}
		return ""
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", nil)
	msg := Banner(addr, BanPost{Pattern: "/foo"}, "secret", req)
	if !strings.Contains(msg, "ban status 200 0") {
		t.Fatalf("unexpected banner reply: %q", msg)
	}
}

func TestPostBanSuccess(t *testing.T) {
	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.url ~ /$" {
			return "200 0       \n"
		}
		return ""
	})
	defer cleanup()

	withServices(t, Services{
		"group1": {Hosts: []string{addr}, Secret: "secret"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", strings.NewReader(`{"Pattern":"/"}`))
	rr := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "service", Value: "group1"}}

	PostBan(rr, req, ps)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var messages Messages
	if err := json.Unmarshal(rr.Body.Bytes(), &messages); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got, ok := messages[addr]; !ok || !strings.Contains(got.Msg, "ban status 200 0") {
		t.Fatalf("unexpected response payload: %#v", messages)
	}
}

func TestBannerSuccessWithVCL(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.http.host == \"example.com\"" {
			return "200 0       \n"
		}
		return ""
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", nil)
	msg := Banner(addr, BanPost{Vcl: `req.http.host == "example.com"`}, "secret", req)
	if !strings.Contains(msg, "ban status 200 0") {
		t.Fatalf("unexpected banner reply: %q", msg)
	}
}

func TestBannerDialError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", nil)
	msg := Banner("127.0.0.1:1", BanPost{Pattern: "/"}, "secret", req)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestBannerAuthError(t *testing.T) {
	t.Parallel()

	lnAddr, lnCleanup := startFakeAdminServerNoChallenge(t)
	defer lnCleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", nil)
	msg := Banner(lnAddr, BanPost{Pattern: "/"}, "secret", req)
	if !strings.Contains(msg, "no challenge code") {
		t.Fatalf("expected auth error, got %q", msg)
	}
}

func TestBannerReadErrorAfterCommand(t *testing.T) {
	t.Parallel()

	addr, cleanup := startFakeAdminServer(t, func(cmd string) string {
		if cmd == "ban req.url ~ /$" {
			return "__CLOSE__"
		}
		return ""
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/v1/service/group1/ban", nil)
	msg := Banner(addr, BanPost{Pattern: "/"}, "secret", req)
	if msg == "" || strings.Contains(msg, "ban status") {
		t.Fatalf("expected read error, got %q", msg)
	}
}
