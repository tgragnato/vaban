package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (appState *application) GetService(
	responseWriter http.ResponseWriter,
	req *http.Request,
	params httprouter.Params,
) {
	service := params.ByName("service")

	serviceConfig, ok := appState.services[service]
	if !ok {
		writePlainError(responseWriter, http.StatusNotFound, "Service could not be found.")

		return
	}

	err := appState.renderer.JSON(responseWriter, http.StatusOK, serviceConfig.Hosts)
	if err != nil {
		writePlainError(responseWriter, http.StatusInternalServerError, err.Error())
		log.Println(err)
	}

	_ = req
}

func (appState *application) GetServices(responseWriter http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	groups := make([]string, 0, len(appState.services))
	for group := range appState.services {
		groups = append(groups, group)
	}

	err := appState.renderer.JSON(responseWriter, http.StatusOK, groups)
	if err != nil {
		writePlainError(responseWriter, http.StatusInternalServerError, err.Error())
		log.Println(err)
	}
}
