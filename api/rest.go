package api

import (
	"errors"
	"github.com/Bnei-Baruch/exec-api/common"
	"github.com/Bnei-Baruch/exec-api/pkg/httputil"
	"github.com/Bnei-Baruch/exec-api/pkg/middleware"
	"github.com/go-cmd/cmd"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strings"
	"time"
)

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

func (a *App) getFile(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	file := vars["file"]

	http.ServeFile(w, r, common.CapturedPath+file)
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
	ep := vars["ep"]

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			httputil.RespondWithError(w, http.StatusBadRequest, "Failed get config")
			return
		}
	}

	var service string
	var args []string
	var id string
	for _, v := range c.Services {
		id = v.ID
		service = v.Name
		args = v.Args
		if len(args) == 0 {
			continue
		}

		if a.Cmd[id] != nil {
			status := a.Cmd[id].Status()
			isruning, err := PidExists(status.PID)
			if err != nil {
				continue
			}
			if isruning {
				continue
			}
		}

		if a.Cmd == nil {
			a.Cmd = make(map[string]*cmd.Cmd)
		}

		if service == "ffmpeg" {
			cmdOptions := cmd.Options{Buffered: false, Streaming: false}
			os.Setenv("FFREPORT", "file=report_"+id+".log:level=32")
			a.Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
		} else {
			a.Cmd[id] = cmd.NewCmd(service, args...)
		}

		a.Cmd[id].Start()

		time.Sleep(2 * time.Second)

		status := a.Cmd[id].Status()
		if status.Exit == 1 {
			continue
		}
	}

	// TODO: return exec report
	httputil.RespondSuccess(w)
}

func (a *App) stopExec(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	ep := vars["ep"]

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			httputil.RespondWithError(w, http.StatusBadRequest, "Failed get config")
			return
		}
	}

	var id string
	for _, v := range c.Services {
		id = v.ID

		if a.Cmd[id] == nil {
			continue
		}

		err := a.Cmd[id].Stop()
		if err != nil {
			continue
		}
	}

	// TODO: return report
	httputil.RespondSuccess(w)
}

func (a *App) startExecByID(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]
	ep := vars["ep"]

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			httputil.RespondWithError(w, http.StatusBadRequest, "Failed get config")
			return
		}
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

	if service == "ffmpeg" {
		cmdOptions := cmd.Options{Buffered: false, Streaming: false}
		os.Setenv("FFREPORT", "file=report_"+id+".log:level=32")
		a.Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
	} else {
		a.Cmd[id] = cmd.NewCmd(service, args...)
	}

	a.Cmd[id].Start()
	status := a.Cmd[id].Status()
	if status.Exit == 1 {
		httputil.RespondWithError(w, http.StatusInternalServerError, "Run Exec Failed")
		return
	}

	httputil.RespondSuccess(w)
}

func (a *App) stopExecByID(w http.ResponseWriter, r *http.Request) {

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

func (a *App) execStatusByID(w http.ResponseWriter, r *http.Request) {

	st := make(map[string]interface{})
	vars := mux.Vars(r)
	id := vars["id"]
	ep := vars["ep"]

	if a.Cmd[id] == nil {
		httputil.RespondWithError(w, http.StatusNotFound, "Nothing to show")
		return
	}

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			httputil.RespondWithError(w, http.StatusBadRequest, "Failed get config")
			return
		}
	}

	for _, i := range c.Services {
		if id == i.ID {
			st["name"] = i.Name
			st["id"] = i.ID
			st["description"] = i.Description
			//st["args"] = i.Args
		}
	}

	if st["name"] == "ffmpeg" {
		progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
		p := <-progress.Start()
		report := strings.Split(p.Stdout[0], "\r")
		n := len(report)
		st["log"] = report[n-1]
	}

	status := a.Cmd[id].Status()
	isruning, err := PidExists(status.PID)
	if err != nil {
		httputil.NewInternalError(err).Abort(w, r)
		return
	}
	st["alive"] = isruning

	//prog := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	//pr := <-prog.Start()
	//for _, line := range pr.Stdout {
	//	args := strings.Split(line, "=")
	//	st[args[0]] = args[1]
	//}

	st["runtime"] = status.Runtime
	st["start"] = status.StartTs
	st["stop"] = status.StopTs

	httputil.RespondWithJSON(w, http.StatusOK, st)
}

func (a *App) execStatus(w http.ResponseWriter, r *http.Request) {

	var id string
	var services []map[string]interface{}
	vars := mux.Vars(r)
	ep := vars["ep"]

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			httputil.RespondWithError(w, http.StatusBadRequest, "Failed get config")
			return
		}
	}

	for _, i := range c.Services {
		st := make(map[string]interface{})
		id = i.ID
		st["name"] = i.Name
		st["id"] = i.ID
		st["description"] = i.Description
		//st["args"] = i.Args

		if a.Cmd[id] == nil {
			st["runtime"] = 0
			st["start"] = 0
			st["stop"] = 0
			st["alive"] = false
		} else {
			if st["name"] == "ffmpeg" {
				progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
				p := <-progress.Start()
				report := strings.Split(p.Stdout[0], "\r")
				n := len(report)
				st["log"] = report[n-1]
			}

			status := a.Cmd[id].Status()
			isruning, err := PidExists(status.PID)
			if err != nil {
				httputil.NewInternalError(err).Abort(w, r)
				return
			}
			st["alive"] = isruning

			//prog := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
			//pr := <-prog.Start()
			//for _, line := range pr.Stdout {
			//	args := strings.Split(line, "=")
			//	st[args[0]] = args[1]
			//}

			st["runtime"] = status.Runtime
			st["start"] = status.StartTs
			st["stop"] = status.StopTs
		}

		services = append(services, st)
	}

	httputil.RespondWithJSON(w, http.StatusOK, services)
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
