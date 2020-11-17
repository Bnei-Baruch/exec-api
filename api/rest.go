package api

import (
	"errors"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/pkg/httputil"
	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
	"github.com/go-cmd/cmd"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strings"
	"syscall"
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

	cmdArgs := os.Getenv("ARGS")
	args := strings.Split(cmdArgs, "!")
	ffpg := []string{"-progress", os.Getenv("PROGRESS") + "stat_" + id + ".log"}
	args = append(ffpg, args...)
	if a.Cmd == nil {
		a.Cmd = make(map[string]*cmd.Cmd)
	}
	a.Cmd[id] = cmd.NewCmd("ffmpeg", args...)
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

func (a *App) execStatus(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	if a.Cmd[id] == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to show")
		return
	}

	status := a.Cmd[id].Status()
	st := &Status{}
	n := len(status.Stderr)
	last := strings.Split(status.Stderr[n-1], "\r")
	st.LastLog = last[len(last)-1]

	httputil.RespondWithJSON(w, http.StatusOK, st)

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

func (a *App) sysStat(w http.ResponseWriter, r *http.Request) {

	temp := cmd.NewCmd("sensors")
	tempStatus := <-temp.Start()

	httputil.RespondWithJSON(w, http.StatusOK, tempStatus.Stdout)

}

func (a *App) prgsTail(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	progress := cmd.NewCmd("tail", "-n", "12", os.Getenv("PROGRESS")+"stat_"+id+".log")
	p := <-progress.Start()

	json := make(map[string]string)

	for _, line := range p.Stdout {
		args := strings.Split(line, "=")
		json[args[0]] = args[1]
	}

	httputil.RespondWithJSON(w, http.StatusOK, json)
}

func logTail(fname string) {
	file, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := make([]byte, 215)
	stat, err := os.Stat(fname)
	start := stat.Size() - 215
	_, err = file.ReadAt(buf, start)
	if err == nil {
		fmt.Printf("%s\n", buf)
	}

}

func PidExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid pid %v", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false, err
	}
	switch errno {
	case syscall.ESRCH:
		return false, nil
	case syscall.EPERM:
		return true, nil
	}
	return false, err
}
