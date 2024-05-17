package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

type HealthStatus struct {
	Admin  string
	Probe  string
	Health string
}
type Backends map[string]HealthStatus
type Servers map[string]Backends

type HealthPost struct {
	Set_health string
}

func GetHealth(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	service := ps.ByName("service")
	backend := ps.ByName("backend")
	if s, ok := services[service]; ok {
		// We need the WaitGroup for some awesome Go concurrency
		var wg sync.WaitGroup
		servers := Servers{}
		for _, server := range s.Hosts {
			// Increment the WaitGroup counter.
			wg.Add(1)
			go func(server string) {
				// Decrement the counter when the goroutine completes.
				defer wg.Done()
				servers[server] = StatusHealth(server, s.Secret, backend)
			}(server)
		}
		wg.Wait()
		err := r.JSON(w, http.StatusOK, servers)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				log.Println(err)
			}
		}
	} else {
		_, err := w.Write([]byte("Service could not be found."))
		if err != nil {
			log.Println(err)
		}
		return
	}
}

func PostHealth(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	service := ps.ByName("service")
	backend := ps.ByName("backend")
	healthpost := HealthPost{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&healthpost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			log.Println(err)
		}
		return
	}
	if healthpost.Set_health == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Set_health is required"))
		if err != nil {
			log.Println(err)
		}
		return
	}
	if s, ok := services[service]; ok {
		// We need the WaitGroup for some awesome Go concurrency
		var wg sync.WaitGroup
		messages := Messages{}
		for _, server := range s.Hosts {
			// Increment the WaitGroup counter.
			wg.Add(1)
			go func(server string) {
				// Decrement the counter when the goroutine completes.
				defer wg.Done()
				message := Message{}
				message.Msg = UpdateHealth(server, s.Secret, backend, healthpost, req)
				messages[server] = message
			}(server)
		}
		wg.Wait()
		err = r.JSON(w, http.StatusOK, messages)
		if err != nil {
			log.Println(err)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		_, err = w.Write([]byte("Service could not be found."))
		if err != nil {
			log.Println(err)
		}
		return
	}
}

func UpdateHealth(server string, secret string, backend string, healthpost HealthPost, req *http.Request) string {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Println(err)
		return err.Error()
	}
	defer conn.Close()
	err = varnishAuth(server, secret, conn)
	if err != nil {
		log.Println(err)
	}
	_, err = conn.Write([]byte("backend.set_health " + backend + " " + healthpost.Set_health + "\n"))
	if err != nil {
		log.Printf("Could not write packet : %s", err.Error())
		return err.Error()
	}
	// again, 64 bytes is enough for this.
	byte_status := make([]byte, 64)
	_, err = conn.Read(byte_status)
	if err != nil {
		log.Printf("Could not read packet : %s", err.Error())
		return err.Error()
	}
	// cast byte to string and only keep the status code (always max 13 char), the rest we dont care.
	status := string(byte_status)[0:12]
	status = strings.Trim(status, " ")
	entry := logrus.WithFields(logrus.Fields{
		"set_health": healthpost.Set_health,
		"backend":    backend,
		"server":     server,
		"status":     status,
	})
	if reqID := req.Header.Get("X-Request-Id"); reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}
	entry.Info("health")
	return "updated with status " + status
}

func StatusHealth(server string, secret string, backend string) Backends {
	backends := Backends{}
	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Println(err)
		return backends
	}
	defer conn.Close()
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
	byte_health := make([]byte, 2048)
	n, err := conn.Read(byte_health)
	if err != nil {
		log.Printf("Could not read packet : %s", err.Error())
		return backends
	}
	status := string(byte_health[:n])
	for _, line := range strings.Split(status, "\n") {
		list := strings.Fields(line)
		if len(list) >= 4 && list[0] != "Backend" {
			hs := HealthStatus{
				Admin:  list[1],
				Probe:  list[2],
				Health: list[3],
			}
			backends[list[0]] = hs
		}
	}
	return backends
}
