package cmd

import (
	"context"
	"github.com/Bnei-Baruch/exec-api/api"
	"github.com/Bnei-Baruch/exec-api/utils"
	"github.com/Bnei-Baruch/exec-api/version"
	"github.com/coreos/go-oidc"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
	"net/http"
	"time"
)

func Init() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, ForceQuote: false, DisableQuote: true})
	if viper.GetString("mqtt.debug") == "true" {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	// Rotating log: rotate daily at 23:59:59 like the pre-2024 builds did, and
	// keep a month of history. HEAD dropped rotation entirely, leaving an
	// append-only file that grows unbounded.
	if logFile := viper.GetString("server.log"); logFile != "" {
		lj := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    1000, // MB — size-based rotation as a backstop
			MaxBackups: 0,    // no count limit; MaxAge governs retention
			MaxAge:     30,   // delete backups older than a month
			LocalTime:  true,
		}
		log.SetOutput(lj)

		// Daily rotation, replacing the gocron job the old builds used.
		go func() {
			for {
				now := time.Now()
				next := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
				if !next.After(now) {
					next = next.AddDate(0, 0, 1)
				}
				time.Sleep(time.Until(next))
				if err := lj.Rotate(); err != nil {
					log.Errorf("log rotate failed: %s", err)
				}
			}
		}()
	} else {
		log.Errorf("server.log not configured, using default stderr")
	}
	log.Infof(" - Starting EXEC API server version %s - ", version.Version)

	// cors
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowMethods = append(corsConfig.AllowMethods, http.MethodDelete)
	corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization")
	corsConfig.AllowAllOrigins = true

	// Authentication
	var oidcIDTokenVerifier *oidc.IDTokenVerifier
	if viper.GetBool("authentication.enable") {
		log.Info("Initializing Auth System")
		issuer := viper.GetString("authentication.issuer")
		oidcProvider, err := oidc.NewProvider(context.TODO(), issuer)
		if err != nil {
			log.Errorf("KC init error: %s", err)
			return
		}
		oidcIDTokenVerifier = oidcProvider.Verifier(&oidc.Config{
			SkipClientIDCheck: true,
		})
	}

	// Setup mqtt
	if err := api.InitMQTT(); err != nil {
		log.Errorf("MQTT Init error: %s", err)
	}

	// Restart managed services that die unexpectedly
	api.StartWatchdog()

	// Setup http
	gin.SetMode(viper.GetString("server.mode"))
	router := gin.New()
	router.Use(
		cors.New(corsConfig),
		utils.MdbLoggerMiddleware(),
		utils.EnvMiddleware(oidcIDTokenVerifier),
		utils.ErrorHandlingMiddleware(),
		utils.AuthenticationMiddleware(),
		utils.RecoveryMiddleware())

	api.SetupRoutes(router)

	srv := &http.Server{
		Addr:    viper.GetString("server.addr"),
		Handler: router,
	}

	// service connections
	log.Infoln("Running application")
	if err := srv.ListenAndServe(); err != nil {
		log.Infof("Server listen: %s", err)
	}
}
