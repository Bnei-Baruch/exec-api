package middleware

import (
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

var requestLog = zerolog.New(os.Stdout).With().Timestamp().Caller().Stack().Logger()

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	zerolog.CallerFieldName = "line"
	zerolog.CallerMarshalFunc = func(file string, line int) string {
		rel := strings.Split(file, "exec-api/")
		return fmt.Sprintf("%s:%d", rel[1], line)
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	log.With().Stack()
}

func LoggingMiddleware(next http.Handler) http.Handler {
	h1 := hlog.NewHandler(requestLog)
	h2 := hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		event := hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("path", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration)

		if rCtx, ok := ContextFromRequest(r); ok {
			event.Str("ip", rCtx.IP)
			if rCtx.IDClaims != nil {
				event.Str("user", rCtx.IDClaims.Email)
			}
		}

		event.Msg("")
	})
	h3 := hlog.RequestIDHandler("request_id", "X-Request-ID")
	return h1(h2(h3(next)))
}

func WriteToLog(action string, msg string) {
	t := time.Now()
	rootPath := common.LogPath
	//timePath := t.Format("2006") + "/" + t.Format("01") + "/" + t.Format("02")
	//fileName := action + "_" + t.Format("15-04-05") + ".log"
	//logPath := rootPath + "/" + timePath + "/" + fileName
	//if _, err := os.Stat(rootPath + "/" + timePath); os.IsNotExist(err) {
	//	os.MkdirAll(rootPath+"/"+timePath, os.ModePerm)
	//}
	ts := t.Format("2006") + "-" + t.Format("01") + "-" + t.Format("02")
	logPath := rootPath + "/" + ts + "_wf.log"
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening file: ", err)
	}
	defer f.Close()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: f})
	log.Info().Str("action", action).Msg(msg)
}
