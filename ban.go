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

type BanPost struct {
	Pattern string
	Vcl     string
}

func PostBan(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	service := ps.ByName("service")
	banpost := BanPost{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&banpost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			log.Println(err)
		}
		return
	}
	if banpost.Pattern == "" && banpost.Vcl == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Pattern or VCL is required"))
		if err != nil {
			log.Println(err)
		}
		return
	} else if banpost.Pattern[0] != '/' {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Pattern must start with a /"))
		if err != nil {
			log.Println(err)
		}
		return
	} else if banpost.Pattern != "" && banpost.Vcl != "" {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write([]byte("Pattern or VCL is required, not both"))
		if err != nil {
			log.Println(err)
		}
		return
	}
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
				message.Msg = Banner(server, banpost, s.Secret, req)
				messages[server] = message
			}(server)
		}
		// Wait for all BANs to complete.
		wg.Wait()
		err = r.JSON(w, http.StatusOK, messages)
		if err != nil {
			log.Println(err)
		}
		return
	} else {
		w.WriteHeader(http.StatusNotFound)
		_, err = w.Write([]byte("Service could not be found."))
		if err != nil {
			log.Println(err)
		}
		return
	}
}

func Banner(server string, banpost BanPost, secret string, req *http.Request) string {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Println(err)
		return err.Error()
	}
	defer conn.Close()
	err = varnishAuth(server, secret, conn)
	if err != nil {
		log.Printf("Authentication error : %s", err.Error())
		return err.Error()
	}
	// sending the magic ban commmand to varnish.
	if banpost.Pattern != "" {
		_, err = conn.Write([]byte("ban req.url ~ " + banpost.Pattern + "$\n"))
	} else {
		_, err = conn.Write([]byte("ban " + banpost.Vcl + "\n"))
	}
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
		"vcl":     banpost.Vcl,
		"pattern": banpost.Pattern,
		"server":  server,
		"status":  status,
	})
	if reqID := req.Header.Get("X-Request-Id"); reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}
	entry.Info("ban")
	return "ban status " + status
}
