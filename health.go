package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

const (
	healthStatusBufferSize = 64
	healthReadBufferSize   = 2048
)

type HealthStatus struct {
	Admin  string
	Probe  string
	Health string
}
type Backends map[string]HealthStatus
type Servers map[string]Backends

type HealthPost struct {
	SetHealth string `json:"setHealth"`
}

func (healthPost *HealthPost) UnmarshalJSON(data []byte) error {
	decoded := map[string]string{}

	err := json.Unmarshal(data, &decoded)
	if err != nil {
		return fmt.Errorf("decode health post: %w", err)
	}

	healthPost.SetHealth = decoded["setHealth"]
	if healthPost.SetHealth == "" {
		healthPost.SetHealth = decoded["Set_health"]
	}

	return nil
}

func collectHealthStatuses(ctx context.Context, serviceConfig Service, backend string) Servers {
	var (
		waitGroup    sync.WaitGroup
		serversMutex sync.Mutex
	)

	servers := Servers{}

	for _, server := range serviceConfig.Hosts {
		waitGroup.Add(1)

		go func(server string) {
			defer waitGroup.Done()

			status := StatusHealth(ctx, server, serviceConfig.Secret, backend)

			serversMutex.Lock()
			servers[server] = status
			serversMutex.Unlock()
		}(server)
	}

	waitGroup.Wait()

	return servers
}

func collectHealthUpdates(
	ctx context.Context,
	serviceConfig Service,
	backend string,
	healthPost HealthPost,
	req *http.Request,
) Messages {
	var (
		waitGroup     sync.WaitGroup
		messagesMutex sync.Mutex
	)

	messages := Messages{}

	for _, server := range serviceConfig.Hosts {
		waitGroup.Add(1)

		go func(server string) {
			defer waitGroup.Done()

			message := Message{Msg: UpdateHealth(ctx, server, serviceConfig.Secret, backend, healthPost, req)}

			messagesMutex.Lock()
			messages[server] = message
			messagesMutex.Unlock()
		}(server)
	}

	waitGroup.Wait()

	return messages
}

func (appState *application) GetHealth(
	responseWriter http.ResponseWriter,
	req *http.Request,
	params httprouter.Params,
) {
	service := params.ByName("service")
	backend := params.ByName("backend")

	serviceConfig, ok := appState.services[service]
	if !ok {
		_, err := responseWriter.Write([]byte("Service could not be found."))
		if err != nil {
			log.Println(err)
		}

		return
	}

	servers := collectHealthStatuses(req.Context(), serviceConfig, backend)

	err := appState.renderer.JSON(responseWriter, http.StatusOK, servers)
	if err != nil {
		writePlainError(responseWriter, http.StatusInternalServerError, err.Error())
	}
}

func (appState *application) PostHealth(
	responseWriter http.ResponseWriter,
	req *http.Request,
	params httprouter.Params,
) {
	service := params.ByName("service")
	backend := params.ByName("backend")
	healthPost := HealthPost{SetHealth: ""}
	decoder := json.NewDecoder(req.Body)

	err := decoder.Decode(&healthPost)
	if err != nil {
		writePlainError(responseWriter, http.StatusInternalServerError, err.Error())

		return
	}

	if healthPost.SetHealth == "" {
		writePlainError(responseWriter, http.StatusBadRequest, "Set_health is required")

		return
	}

	serviceConfig, ok := appState.services[service]
	if !ok {
		writePlainError(responseWriter, http.StatusNotFound, "Service could not be found.")

		return
	}

	messages := collectHealthUpdates(req.Context(), serviceConfig, backend, healthPost, req)

	err = appState.renderer.JSON(responseWriter, http.StatusOK, messages)
	if err != nil {
		log.Println(err)
	}
}

func UpdateHealth(
	ctx context.Context,
	server, secret, backend string,
	healthPost HealthPost,
	req *http.Request,
) string {
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		log.Println(err)

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
		log.Println(err)
	}

	_, err = conn.Write([]byte("backend.set_health " + backend + " " + healthPost.SetHealth + "\n"))
	if err != nil {
		log.Printf("Could not write packet : %s", err.Error())

		return err.Error()
	}

	statusBytes := make([]byte, healthStatusBufferSize)

	_, err = conn.Read(statusBytes)
	if err != nil {
		log.Printf("Could not read packet : %s", err.Error())

		return err.Error()
	}

	status := strings.Trim(string(statusBytes)[0:12], " ")

	entry := logrus.WithFields(logrus.Fields{
		"set_health": healthPost.SetHealth,
		"backend":    backend,
		"server":     server,
		"status":     status,
	})
	if reqID := req.Header.Get("X-Request-ID"); reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}

	entry.Info("health")

	return "updated with status " + status
}

func StatusHealth(ctx context.Context, server, secret, backend string) Backends {
	backends := Backends{}

	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		log.Println(err)

		return backends
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			log.Println(closeErr)
		}
	}()

	err = varnishAuth(server, secret, conn)
	if err != nil {
		log.Println(err)
	}

	if backend == "" {
		_, err = conn.Write([]byte("backend.list\n"))
	} else {
		_, err = conn.Write([]byte("backend.list " + backend + "\n"))
	}

	if err != nil {
		log.Printf("Could not write packet : %s", err.Error())

		return backends
	}

	byteHealth := make([]byte, healthReadBufferSize)

	bytesRead, err := conn.Read(byteHealth)
	if err != nil {
		log.Printf("Could not read packet : %s", err.Error())

		return backends
	}

	status := string(byteHealth[:bytesRead])
	for line := range strings.SplitSeq(status, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] != "Backend" {
			hs := HealthStatus{
				Admin:  fields[1],
				Probe:  fields[2],
				Health: fields[3],
			}
			backends[fields[0]] = hs
		}
	}

	return backends
}
