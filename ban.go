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

const bannerStatusBufferSize = 64

type BanPost struct {
	Pattern string `json:"pattern"`
	Vcl     string `json:"vcl"`
}

func (banPost *BanPost) UnmarshalJSON(data []byte) error {
	decoded := map[string]string{}

	err := json.Unmarshal(data, &decoded)
	if err != nil {
		return fmt.Errorf("decode ban post: %w", err)
	}

	banPost.Pattern = decoded["pattern"]
	if banPost.Pattern == "" {
		banPost.Pattern = decoded["Pattern"]
	}

	banPost.Vcl = decoded["vcl"]
	if banPost.Vcl == "" {
		banPost.Vcl = decoded["Vcl"]
	}

	return nil
}

type banValidationResult struct {
	message    string
	statusCode int
}

func writePlainError(responseWriter http.ResponseWriter, statusCode int, message string) {
	responseWriter.WriteHeader(statusCode)

	_, err := responseWriter.Write([]byte(message))
	if err != nil {
		log.Println(err)
	}
}

func validateBanPost(banPost BanPost) banValidationResult {
	if banPost.Pattern == "" && banPost.Vcl == "" {
		return banValidationResult{message: "Pattern or VCL is required", statusCode: http.StatusBadRequest}
	}

	if banPost.Pattern != "" && banPost.Pattern[0] != '/' {
		return banValidationResult{message: "Pattern must start with a /", statusCode: http.StatusBadRequest}
	}

	if banPost.Pattern != "" && banPost.Vcl != "" {
		return banValidationResult{message: "Pattern or VCL is required, not both", statusCode: http.StatusBadRequest}
	}

	return banValidationResult{message: "", statusCode: 0}
}

func collectBanMessages(ctx context.Context, serviceConfig Service, banPost BanPost, req *http.Request) Messages {
	var (
		waitGroup     sync.WaitGroup
		messagesMutex sync.Mutex
	)

	messages := Messages{}

	for _, server := range serviceConfig.Hosts {
		waitGroup.Add(1)

		go func(server string) {
			defer waitGroup.Done()

			message := Message{Msg: Banner(ctx, server, banPost, serviceConfig.Secret, req)}

			messagesMutex.Lock()
			messages[server] = message
			messagesMutex.Unlock()
		}(server)
	}

	waitGroup.Wait()

	return messages
}

func (appState *application) PostBan(responseWriter http.ResponseWriter, req *http.Request, params httprouter.Params) {
	service := params.ByName("service")

	serviceConfig, ok := appState.services[service]
	if !ok {
		writePlainError(responseWriter, http.StatusNotFound, "Service could not be found.")

		return
	}

	banPost := BanPost{Pattern: "", Vcl: ""}
	decoder := json.NewDecoder(req.Body)

	err := decoder.Decode(&banPost)
	if err != nil {
		writePlainError(responseWriter, http.StatusInternalServerError, err.Error())

		return
	}

	validation := validateBanPost(banPost)
	if validation.message != "" {
		writePlainError(responseWriter, validation.statusCode, validation.message)

		return
	}

	messages := collectBanMessages(req.Context(), serviceConfig, banPost, req)

	err = appState.renderer.JSON(responseWriter, http.StatusOK, messages)
	if err != nil {
		log.Println(err)
	}
}

func Banner(ctx context.Context, server string, banPost BanPost, secret string, req *http.Request) string {
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
		log.Printf("Authentication error : %s", err.Error())

		return err.Error()
	}

	if banPost.Pattern != "" {
		_, err = conn.Write([]byte("ban req.url ~ " + banPost.Pattern + "$\n"))
	} else {
		_, err = conn.Write([]byte("ban " + banPost.Vcl + "\n"))
	}

	if err != nil {
		log.Printf("Could not write packet : %s", err.Error())

		return err.Error()
	}

	statusBytes := make([]byte, bannerStatusBufferSize)

	_, err = conn.Read(statusBytes)
	if err != nil {
		log.Printf("Could not read packet : %s", err.Error())

		return err.Error()
	}

	status := strings.Trim(string(statusBytes)[0:12], " ")

	entry := logrus.WithFields(logrus.Fields{
		"vcl":     banPost.Vcl,
		"pattern": banPost.Pattern,
		"server":  server,
		"status":  status,
	})
	if reqID := req.Header.Get("X-Request-ID"); reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}

	entry.Info("ban")

	return "ban status " + status
}
