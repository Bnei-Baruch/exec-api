package api

import (
	"context"
	"net/http"

	"github.com/coreos/go-oidc"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"

	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
)

type App struct {
	Router        *mux.Router
	Handler       http.Handler
	tokenVerifier *oidc.IDTokenVerifier
}

func (a *App) Initialize(accountsUrl string, skipAuth bool) {
	log.Info().Msg("initializing app")

	a.InitApp(accountsUrl, skipAuth)
}

func (a *App) InitApp(accountsUrl string, skipAuth bool) {

	a.Router = mux.NewRouter()
	a.initializeRoutes()

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
	a.Router.HandleFunc("/data", a.getData).Methods("GET")
}
