package api

import (
	"errors"
	"github.com/Bnei-Baruch/exec-api/pkg/httputil"
	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
	"github.com/go-cmd/cmd"
	"github.com/gorilla/mux"
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

func (a *App) isAlive(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			httputil.NewInternalError(err).Abort(w, r)
			return
		}
		if isruning {
			httputil.RespondSuccess(w)
			return
		}
	}

	httputil.RespondWithError(w, http.StatusNotFound, "died")
}

func (a *App) startExec(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	c, err := getConf()
	if err != nil {
		httputil.RespondWithError(w, http.StatusBadRequest, "Failed get config")
		return
	}

	var service string
	var args []string
	for _, v := range c.Services {
		if v.ID == id {
			service = v.Name
			args = v.Args
			break
		}
	}

	if len(args) == 0 {
		httputil.RespondWithError(w, http.StatusBadRequest, "Id not found")
		return
	}

	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			httputil.NewInternalError(err).Abort(w, r)
			return
		}
		if isruning {
			httputil.RespondWithError(w, http.StatusBadRequest, "Already executed")
			return
		}
	}

	if a.Cmd == nil {
		a.Cmd = make(map[string]*cmd.Cmd)
	}

	cmdOptions := cmd.Options{
		Buffered:  false,
		Streaming: false,
	}

	os.Setenv("FFREPORT", "file=report_"+id+".log:level=32")
	a.Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
	a.Cmd[id].Start()
	status := a.Cmd[id].Status()
	if status.Exit == 1 {
		httputil.RespondWithError(w, http.StatusInternalServerError, "Run Exec Failed")
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) stopExec(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	if a.Cmd[id] == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to stop")
		return
	}

	err := a.Cmd[id].Stop()
	if err != nil {
		httputil.NewUnauthorizedError(err).Abort(w, r)
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) cmdStat(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	if a.Cmd[id] == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to show")
		return
	}

	status := a.Cmd[id].Status()

	httputil.RespondWithJSON(w, http.StatusOK, status)

}

func (a *App) execStatus(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	if a.Cmd[id] == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to show")
		return
	}

	progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
	p := <-progress.Start()

	report := strings.Split(p.Stdout[0], "\r")

	st := &Status{}
	n := len(report)
	st.LastLog = report[n-1]

	httputil.RespondWithJSON(w, http.StatusOK, st)

}

func (a *App) sysStat(w http.ResponseWriter, r *http.Request) {

	temp := cmd.NewCmd("sensors")
	tempStatus := <-temp.Start()

	httputil.RespondWithJSON(w, http.StatusOK, tempStatus.Stdout)

}

func (a *App) getProgress(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	progress := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	p := <-progress.Start()

	ffjson := make(map[string]string)

	for _, line := range p.Stdout {
		args := strings.Split(line, "=")
		ffjson[args[0]] = args[1]
	}

	httputil.RespondWithJSON(w, http.StatusOK, ffjson)
}

func (a *App) getReport(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	progress := cmd.NewCmd("cat", "report_"+id+".log")
	p := <-progress.Start()

	httputil.RespondWithJSON(w, http.StatusOK, p)

}
