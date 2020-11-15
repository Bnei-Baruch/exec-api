package api

import (
	"errors"
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

func (a *App) stopExec(w http.ResponseWriter, r *http.Request) {

	if a.Cmd == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to stop")
	}

	err := a.Cmd.Stop()
	if err != nil {
		httputil.NewUnauthorizedError(err).Abort(w, r)
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) statExec(w http.ResponseWriter, r *http.Request) {

	if a.Cmd == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to show")
	}

	status := a.Cmd.Status()
	httputil.RespondWithJSON(w, http.StatusOK, status)

}

func (a *App) runExec(w http.ResponseWriter, r *http.Request) {

	if a.Cmd != nil {
		httputil.RespondWithError(w, http.StatusBadRequest, "Already executed")
	}

	cmdArgs := os.Getenv("ARGS")
	args := strings.Split(cmdArgs, " ")
	a.Cmd = cmd.NewCmd("ffmpeg", args...)

	a.Cmd.Start()

	httputil.RespondSuccess(w)
}
