package api

import (
	"context"
	"fmt"
	"github.com/coreos/go-oidc"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-cmd/cmd"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"

	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
)

type App struct {
	Router        *mux.Router
	Handler       http.Handler
	tokenVerifier *oidc.IDTokenVerifier
	Cmd           map[string]*cmd.Cmd
	Msg           mqtt.Client
}

func (a *App) Initialize(accountsUrl string, skipAuth bool) {
	log.Info().Msg("initializing app")

	a.InitApp(accountsUrl, skipAuth)
}

func (a *App) InitApp(accountsUrl string, skipAuth bool) {

	a.Router = mux.NewRouter()
	a.initializeRoutes()
	a.initMQTT()

	if !skipAuth {
		a.initOidc(accountsUrl)
	}

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders: []string{"Origin", "Accept", "Content-Type", "X-Requested-With", "Authorization"},
	})

	a.Handler = middleware.ContextMiddleware(
		middleware.LoggingMiddleware(
			middleware.RecoveryMiddleware(
				middleware.RealIPMiddleware(
					corsMiddleware.Handler(
						middleware.AuthenticationMiddleware(a.tokenVerifier, skipAuth)(
							a.Router))))))
}

func (a *App) initOidc(issuer string) {
	oidcProvider, err := oidc.NewProvider(context.TODO(), issuer)
	if err != nil {
		log.Fatal().Err(err).Msg("oidc.NewProvider")
	}

	a.tokenVerifier = oidcProvider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})
}

func (a *App) Run(listenAddr string) {
	addr := listenAddr
	if addr == "" {
		addr = ":8080"
	}

	log.Info().Msgf("app run %s", addr)
	if err := http.ListenAndServe(addr, a.Handler); err != nil {
		log.Fatal().Err(err).Msg("http.ListenAndServe")
	}
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/{ep}/test", a.getData).Methods("GET")
	a.Router.HandleFunc("/{ep}/sysstat", a.sysStat).Methods("GET")
	a.Router.HandleFunc("/{ep}/status", a.execStatus).Methods("GET")
	a.Router.HandleFunc("/{ep}/start", a.startExec).Methods("GET")
	a.Router.HandleFunc("/{ep}/stop", a.stopExec).Methods("GET")
	a.Router.HandleFunc("/{ep}/start/{id}", a.startExecByID).Methods("GET")
	a.Router.HandleFunc("/{ep}/stop/{id}", a.stopExecByID).Methods("GET")
	a.Router.HandleFunc("/{ep}/status/{id}", a.execStatusByID).Methods("GET")
	a.Router.HandleFunc("/{ep}/cmdstat/{id}", a.cmdStat).Methods("GET")
	a.Router.HandleFunc("/{ep}/progress/{id}", a.getProgress).Methods("GET")
	a.Router.HandleFunc("/{ep}/report/{id}", a.getReport).Methods("GET")
	a.Router.HandleFunc("/{ep}/alive/{id}", a.isAlive).Methods("GET")
}

func (a *App) initMQTT() {
	if os.Getenv("MQTT_URL") != "" {
		server := os.Getenv("MQTT_URL")
		username := os.Getenv("MQTT_USER")
		password := os.Getenv("MQTT_PASS")
		ep := os.Getenv("MQTT_EP")

		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("ssl://%s", server))
		opts.SetClientID(ep + "-exec_mqtt_client")
		opts.SetUsername(username)
		opts.SetPassword(password)
		//opts.SetDefaultPublishHandler(messagePubHandler)
		opts.OnConnect = connectHandler
		opts.OnConnectionLost = connectLostHandler
		a.Msg = mqtt.NewClient(opts)
		if token := a.Msg.Connect(); token.Wait() && token.Error() != nil {
			err := token.Error()
			log.Fatal().Err(err).Msg("initialize mqtt listener")
		}

		if token := a.Msg.Subscribe("exec/service/"+ep+"/#", byte(1), a.MsgHandler); token.Wait() && token.Error() != nil {
			log.Fatal().Err(token.Error()).Msg("mqtt.client Subscribe")
		}

		if token := a.Msg.Subscribe("kli/exec/service/"+ep+"/#", byte(1), a.MsgHandler); token.Wait() && token.Error() != nil {
			log.Fatal().Err(token.Error()).Msg("mqtt.client Subscribe")
		}
	}
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected to MQTT")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Fatal().Err(err).Msg("Connect lost")
}
