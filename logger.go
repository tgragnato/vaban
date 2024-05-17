package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/sirupsen/logrus"
)

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
	log.Formatter = &logrus.TextFormatter{}
	name := "vaban"
	return &Middleware{Logger: log, Name: name}
}

func (l *Middleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	start := time.Now()
	next(rw, r)
	latency := time.Since(start)
	res := rw.(negroni.ResponseWriter)
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	var clientip string
	if forwarded != "" {
		clientip = forwarded
	} else {
		clientip = strings.Split(r.RemoteAddr, ":")[0]
	}
	entry := l.Logger.WithFields(logrus.Fields{
		"request": r.RequestURI,
		"method":  r.Method,
		"remote":  clientip,
		"status":  res.Status(),
		"took":    latency,
		fmt.Sprintf("measure#%s.latency", l.Name): latency.Nanoseconds(),
	})
	if reqID := r.Header.Get("X-Request-Id"); reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}
	entry.Info("request")
}
