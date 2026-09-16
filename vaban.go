/* Vaban - The Simple Varnish Ban REST Api. <benjamin@martensson.io> */
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/codegangsta/negroni"
	"github.com/goccy/go-yaml"
	"github.com/julienschmidt/httprouter"
	"github.com/pilu/xrequestid"
	"github.com/thoas/stats"
	"github.com/unrolled/render"
)

type Message struct {
	Msg string
}
type Messages map[string]Message

type Service struct {
	Hosts  []string
	Secret string
}
type Services map[string]Service

type application struct {
	services Services
	renderer *render.Render
}

func newApplication() *application {
	var options render.Options

	options.IndentJSON = true

	return &application{
		services: Services{},
		renderer: render.New(options),
	}
}

const requestIDLength = 8

func (appState *application) initialize() *negroni.Negroni {
	vabanStats := stats.New()
	app := negroni.New(
		negroni.NewRecovery(),
		NewLogger(),
		xrequestid.New(requestIDLength),
	)

	router := httprouter.New()
	router.GET("/", func(responseWriter http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		statsData := vabanStats.Data()

		err := appState.renderer.JSON(responseWriter, http.StatusOK, statsData)
		if err != nil {
			responseWriter.WriteHeader(http.StatusInternalServerError)

			_, err = responseWriter.Write([]byte(err.Error()))
			if err != nil {
				log.Println(err)
			}
		}
	})
	router.GET("/v1/services", appState.GetServices)
	router.GET("/v1/service/:service", appState.GetService)
	router.GET("/v1/service/:service/ping", appState.GetPing)
	router.GET("/v1/service/:service/health", appState.GetHealth)
	router.GET("/v1/service/:service/health/:backend", appState.GetHealth)
	router.POST("/v1/service/:service/health/:backend", appState.PostHealth)
	router.POST("/v1/service/:service/ban", appState.PostBan)
	// add router and clear mux.context values at the end of request life-times
	app.UseHandler(router)

	return app
}

func main() {
	port := flag.String("p", "4000", "Listen on this port. (default 4000)")
	config := flag.String("f", "config.yml", "Path to config. (default config.yml)")

	flag.Parse()

	file, err := os.ReadFile(*config)
	if err != nil {
		log.Println(err)

		return
	}

	appState := newApplication()

	err = yaml.Unmarshal(file, &appState.services)
	if err != nil {
		log.Println("Problem parsing config: ", err)

		return
	}

	app := appState.initialize()

	log.Println("Starting vaban on :" + *port)
	app.Run(":" + *port)
}
