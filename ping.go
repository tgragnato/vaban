package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
)

const pingResponseBufferSize = 32

func Pinger(ctx context.Context, server, secret string) string {
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return err.Error()
	}

	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			log.Println(closeErr)
		}
	}()

	err = varnishAuth(server, secret, conn)
	if err != nil {
		return err.Error()
	}

	_, err = conn.Write([]byte("ping\n"))
	if err != nil {
		return err.Error()
	}

	pong := make([]byte, pingResponseBufferSize)

	_, err = conn.Read(pong)
	if err != nil {
		return err.Error()
	}

	status := string(pong)[13:32]
	status = strings.Trim(status, " ")

	return status
}

func (appState *application) GetPing(responseWriter http.ResponseWriter, req *http.Request, params httprouter.Params) {
	service := params.ByName("service")

	serviceConfig, ok := appState.services[service]
	if !ok {
		writePlainError(responseWriter, http.StatusNotFound, "Service could not be found.")

		return
	}

	var (
		waitGroup     sync.WaitGroup
		messagesMutex sync.Mutex
	)

	messages := Messages{}

	for _, server := range serviceConfig.Hosts {
		waitGroup.Add(1)

		go func(server string) {
			defer waitGroup.Done()

			message := Message{Msg: Pinger(req.Context(), server, serviceConfig.Secret)}

			messagesMutex.Lock()
			messages[server] = message
			messagesMutex.Unlock()
		}(server)
	}

	waitGroup.Wait()

	err := appState.renderer.JSON(responseWriter, http.StatusOK, messages)
	if err != nil {
		writePlainError(responseWriter, http.StatusInternalServerError, err.Error())
		log.Println(err)
	}
}
