package main

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
)

const (
	fakeChallenge   = "abcdefghijklmnopqrstuvwxyzabcdef"
	testGroup1      = "group1"
	testMissing     = "missing"
	testSecret      = "secret"
	testServiceKey  = "service"
	testBackendKey  = "backend"
	testBackendName = "be1"
	testHealthSick  = "sick"
	testClosedAddr  = "127.0.0.1:1"
	testCloseToken  = "__CLOSE__"
	testOKResponse  = "200 0       \n"
	testPingCommand = "ping"
	testBackendList = "backend.list"
)

type fakeAdminServer struct {
	address string
	cleanup func()
}

func newTestApplication(providedServices Services) *application {
	appState := newApplication()
	appState.services = providedServices

	return appState
}

func startFakeAdminServer(t *testing.T, onCommand func(string) string) fakeAdminServer {
	t.Helper()

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	done := make(chan struct{})
	go acceptFakeConnections(listener, done, onCommand)

	return fakeAdminServer{
		address: listener.Addr().String(),
		cleanup: func() {
			close(done)

			_ = listener.Close()
		},
	}
}

func startFakeAdminServerNoChallenge(t *testing.T) fakeAdminServer {
	t.Helper()

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	done := make(chan struct{})
	go acceptNoChallengeConnections(listener, done)

	return fakeAdminServer{
		address: listener.Addr().String(),
		cleanup: func() {
			close(done)

			_ = listener.Close()
		},
	}
}

func acceptFakeConnections(listener net.Listener, done <-chan struct{}, onCommand func(string) string) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-done:
				return
			default:
				return
			}
		}

		go handleFakeConnection(conn, onCommand)
	}
}

func handleFakeConnection(connection net.Conn, onCommand func(string) string) {
	defer connection.Close()

	_, _ = connection.Write([]byte("107 0\nWelcome\n" + fakeChallenge + "\n"))

	reader := bufio.NewReader(connection)

	authLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	if !strings.HasPrefix(authLine, "auth ") {
		return
	}

	_, _ = connection.Write([]byte("200 0 \n"))

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		resp := onCommand(strings.TrimSpace(line))
		if resp == testCloseToken {
			return
		}

		if resp != "" {
			_, _ = connection.Write([]byte(resp))
		}
	}
}

func acceptNoChallengeConnections(listener net.Listener, done <-chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-done:
				return
			default:
				return
			}
		}

		go func(connection net.Conn) {
			defer connection.Close()

			_, _ = connection.Write([]byte("107 0\nno challenge here\n"))
		}(conn)
	}
}
