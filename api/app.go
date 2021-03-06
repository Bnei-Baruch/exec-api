package api

import (
	"context"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
	"github.com/coreos/go-oidc"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-cmd/cmd"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
	"net/http"
)

type App struct {
	Router        *mux.Router
	Handler       http.Handler
	tokenVerifier *oidc.IDTokenVerifier
	Cmd           map[string]*cmd.Cmd
	Msg           mqtt.Client
}

func (a *App) Initialize(accountsUrl string, skipAuth bool) {
	middleware.InitLog()
	log.Info().Str("source", "APP").Msg("initializing app")
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
		log.Fatal().Str("source", "APP").Err(err).Msg("oidc.NewProvider")
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

	log.Info().Str("source", "APP").Msgf("app run %s", addr)
	if err := http.ListenAndServe(addr, a.Handler); err != nil {
		log.Fatal().Str("source", "APP").Err(err).Msg("http.ListenAndServe")
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
	a.Router.HandleFunc("/get/{file}", a.getFile).Methods("GET")
}

func (a *App) initMQTT() {
	if common.SERVER != "" {
		//a.InitLogMQTT()
		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("ssl://%s", common.SERVER))
		opts.SetClientID(common.EP + "-exec_mqtt_client")
		opts.SetUsername(common.USERNAME)
		opts.SetPassword(common.PASSWORD)
		opts.SetAutoReconnect(true)
		opts.SetOnConnectHandler(a.SubMQTT)
		opts.SetConnectionLostHandler(a.LostMQTT)
		a.Msg = mqtt.NewClient(opts)
		if token := a.Msg.Connect(); token.Wait() && token.Error() != nil {
			err := token.Error()
			log.Fatal().Str("source", "MQTT").Err(err).Msg("initialize mqtt listener")
		}
	}
}
