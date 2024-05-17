package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func GetService(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	service := ps.ByName("service")

	if s, ok := services[service]; ok {
		err := r.JSON(w, http.StatusOK, s.Hosts)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				log.Println(err)
			}
		}
		return
	} else {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte("Service could not be found."))
		if err != nil {
			log.Println(err)
		}
		return
	}
}

func GetServices(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var keys []string
	for k := range services {
		keys = append(keys, k)
	}
	err := r.JSON(w, http.StatusOK, keys)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			log.Println(err)
		}
	}
}
