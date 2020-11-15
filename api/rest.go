package api

import (
	"errors"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/pkg/httputil"
	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
	"github.com/go-cmd/cmd"
	"net/http"
	"os"
	"strings"
)

type Proc struct {
	Stat *cmd.Status
	Cmd  *cmd.Cmd
}

func (a *App) getData(w http.ResponseWriter, r *http.Request) {

	// Check role
	authRoot := middleware.CheckRole("auth_root", r)
	if !authRoot {
		e := errors.New("bad permission")
		httputil.NewUnauthorizedError(e).Abort(w, r)
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) stopData(w http.ResponseWriter, r *http.Request) {

	err := a.Cmd.Stop()
	if err != nil {
		httputil.NewUnauthorizedError(err).Abort(w, r)
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) statData(w http.ResponseWriter, r *http.Request) {

	fmt.Println(a.Cmd.Status())

	httputil.RespondSuccess(w)
}

func (a *App) execData(w http.ResponseWriter, r *http.Request) {

	cmdArgs := os.Getenv("ARGS")
	args := strings.Split(cmdArgs, " ")
	a.Cmd = cmd.NewCmd("ffmpeg", args...)

	a.Cmd.Start()

	httputil.RespondSuccess(w)
}
