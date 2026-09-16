package main

import (
	"net"
	"strings"
	"testing"
)

func TestVarnishAuthSuccess(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	go func() {
		defer serverConn.Close()

		_, _ = serverConn.Write([]byte("107 0\n" + fakeChallenge + "\n"))

		buf := make([]byte, 256)

		_, err := serverConn.Read(buf)
		if err != nil {
			return
		}

		_, _ = serverConn.Write([]byte("200 0 \n"))
	}()

	err := varnishAuth("127.0.0.1:6082", "secret", clientConn)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestVarnishAuthWithoutChallenge(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	go func() {
		defer serverConn.Close()

		_, _ = serverConn.Write([]byte("107 0\nno challenge here\n"))
	}()

	err := varnishAuth("example", "secret", clientConn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "no challenge code") {
		t.Fatalf("unexpected error: %v", err)
	}
}
