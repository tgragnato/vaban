package main

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

const fakeChallenge = "abcdefghijklmnopqrstuvwxyzabcdef"

func withServices(t *testing.T, s Services) {
	t.Helper()
	old := services
	services = s
	t.Cleanup(func() {
		services = old
	})
}

// startFakeAdminServer starts a minimal fake Varnish admin TCP server.
func startFakeAdminServer(t *testing.T, onCommand func(string) string) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}

			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("107 0\nWelcome\n" + fakeChallenge + "\n"))

				reader := bufio.NewReader(c)
				authLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if !strings.HasPrefix(authLine, "auth ") {
					return
				}
				_, _ = c.Write([]byte("200 0 \n"))

				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					resp := onCommand(strings.TrimSpace(line))
					if resp == "__CLOSE__" {
						return
					}
					if resp != "" {
						_, _ = c.Write([]byte(resp))
					}
				}
			}(conn)
		}
	}()

	cleanup := func() {
		close(done)
		_ = ln.Close()
	}

	return ln.Addr().String(), cleanup
}

func startFakeAdminServerNoChallenge(t *testing.T) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}

			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("107 0\nno challenge here\n"))
			}(conn)
		}
	}()

	cleanup := func() {
		close(done)
		_ = ln.Close()
	}

	return ln.Addr().String(), cleanup
}
