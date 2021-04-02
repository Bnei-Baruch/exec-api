package middleware

import (
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	ConsoleLoggingEnabled bool
	EncodeLogsAsJson      bool
	FileLoggingEnabled    bool
	Directory             string
	Filename              string
	MaxSize               int
	MaxBackups            int
	MaxAge                int
	LocalTime             bool
}

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

func InitLog() {
	c := Config{
		ConsoleLoggingEnabled: false,
		FileLoggingEnabled:    true,
		EncodeLogsAsJson:      true,
		LocalTime:             true,
		Directory:             common.LogPath,
		Filename:              "latest.log",
		MaxSize:               1000,
		MaxBackups:            1000,
		MaxAge:                1,
	}

	var writers []io.Writer

	if c.ConsoleLoggingEnabled {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr})
	}
	if c.FileLoggingEnabled {
		writers = append(writers, newRollingFile(c))
	}
	mw := io.MultiWriter(writers...)

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	//zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.MessageFieldName = "msg"
	log.Logger = zerolog.New(mw).With().Timestamp().Logger()
}

func newRollingFile(c Config) io.Writer {
	if err := os.MkdirAll(c.Directory, 0744); err != nil {
		log.Error().Err(err).Str("path", c.Directory).Msg("can't create log directory")
		return nil
	}

	return &lumberjack.Logger{
		Filename:   path.Join(c.Directory, c.Filename),
		MaxBackups: c.MaxBackups,
		MaxSize:    c.MaxSize,
		MaxAge:     c.MaxAge,
		LocalTime:  c.LocalTime,
	}
}
