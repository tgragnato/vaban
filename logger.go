package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/sirupsen/logrus"
)

const statusField = "status"

// Middleware is a middleware handler that logs the request as it goes in and the response as it goes out.
type Middleware struct {
	// Logger is the log.Logger instance used to log messages with the Logger middleware
	Logger *logrus.Logger
	// Name is the name of the application as recorded in latency metrics
	Name string
}

func NewLogger() *Middleware {
	log := logrus.New()
	log.Level = logrus.InfoLevel
	log.Formatter = new(logrus.TextFormatter)
	name := "vaban"

	return &Middleware{Logger: log, Name: name}
}

func (l *Middleware) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request, next http.HandlerFunc) {
	start := time.Now()

	next(responseWriter, request)

	latency := time.Since(start)

	response, ok := responseWriter.(negroni.ResponseWriter)
	if !ok {
		return
	}

	forwarded := request.Header.Get("X-Forwarded-For")

	var clientip string
	if forwarded != "" {
		clientip = forwarded
	} else {
		clientip = strings.Split(request.RemoteAddr, ":")[0]
	}

	entry := l.Logger.WithFields(logrus.Fields{
		"request":   request.RequestURI,
		"method":    request.Method,
		"remote":    clientip,
		statusField: response.Status(),
		"took":      latency,
		fmt.Sprintf("measure#%s.latency", l.Name): latency.Nanoseconds(),
	})
	if reqID := request.Header.Get("X-Request-ID"); reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}

	entry.Info("request")
}
