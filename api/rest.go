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

type Status struct {
	LastLog string
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

func (a *App) runExec(w http.ResponseWriter, r *http.Request) {

	if a.Cmd != nil {
		status := a.Cmd.Status()
		if !status.Complete {
			httputil.RespondWithError(w, http.StatusBadRequest, "Already executed")
			return
		}
	}

	cmdArgs := os.Getenv("ARGS")
	args := strings.Split(cmdArgs, "!")
	a.Cmd = cmd.NewCmd("ffmpeg", args...)
	a.Cmd.Start()
	status := a.Cmd.Status()
	if status.Exit == 1 {
		httputil.RespondWithError(w, http.StatusInternalServerError, "Run Exec Failed")
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) stopExec(w http.ResponseWriter, r *http.Request) {

	if a.Cmd == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to stop")
		return
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
		return
	}

	status := a.Cmd.Status()
	st := &Status{}
	n := len(status.Stderr)
	last := strings.Split(status.Stderr[n-1], "\r")
	st.LastLog = last[len(last)-1]

	httputil.RespondWithJSON(w, http.StatusOK, st)

}

func (a *App) statCmd(w http.ResponseWriter, r *http.Request) {

	if a.Cmd == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to show")
		return
	}

	status := a.Cmd.Status()

	httputil.RespondWithJSON(w, http.StatusOK, status)

}

func (a *App) statOs(w http.ResponseWriter, r *http.Request) {

	temp := cmd.NewCmd("sensors")
	tempStatus := <-temp.Start()

	httputil.RespondWithJSON(w, http.StatusOK, tempStatus.Stdout)

}
