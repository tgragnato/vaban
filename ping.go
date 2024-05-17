package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
)

func Pinger(server string, secret string) string {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	err = varnishAuth(server, secret, conn)
	if err != nil {
		return err.Error()
	}
	_, err = conn.Write([]byte("ping\n"))
	if err != nil {
		return err.Error()
	}
	pong := make([]byte, 32)
	_, err = conn.Read(pong)
	if err != nil {
		return err.Error()
	}
	status := string(pong)[13:32]
	status = strings.Trim(status, " ")
	return status
}

func GetPing(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	service := ps.ByName("service")

	if s, ok := services[service]; ok {
		// We need the WaitGroup for some awesome Go concurrency of our BANs
		var wg sync.WaitGroup
		messages := Messages{}
		for _, server := range s.Hosts {
			// Increment the WaitGroup counter.
			wg.Add(1)
			go func(server string) {
				// Decrement the counter when the goroutine completes.
				defer wg.Done()
				message := Message{}
				message.Msg = Pinger(server, s.Secret)
				messages[server] = message
			}(server)
		}
		// Wait for all PINGs to complete.
		wg.Wait()
		err := r.JSON(w, http.StatusOK, messages)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				log.Println(err)
			}
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte("Service could not be found."))
		if err != nil {
			log.Println(err)
		}
		return
	}
}
