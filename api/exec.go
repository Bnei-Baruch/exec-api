package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	wf "github.com/Bnei-Baruch/exec-api/pkg/workflow"
	"github.com/go-cmd/cmd"
	"github.com/rs/zerolog/log"
	"os"
	"regexp"
	"strings"
	"time"
)

func (a *App) isAliveMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Error"
			a.SendRespond(id, rp)
			return
		}
		if isruning {
			rp.Message = "Alive"
			a.SendRespond(id, rp)
			return
		}
	}

	rp.Message = "Died"
	a.SendRespond(id, rp)
}

func (a *App) startExecMqtt(p string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond("false", rp)
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
	rp.Message = "Success"
	a.SendRespond("false", rp)
}

func (a *App) stopExecMqtt(p string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond("false", rp)
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

	removeProgress("stat_" + id + ".log")

	// TODO: return report
	rp.Message = "Success"
	a.SendRespond("false", rp)
}

func (a *App) startExecMqttByID(p string, id string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(id, rp)
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
		rp.Error = fmt.Errorf("error")
		rp.Message = "ID not found"
		a.SendRespond(id, rp)
		return
	}

	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Internal error"
			a.SendRespond(id, rp)
			return
		}
		if isruning {
			rp.Error = fmt.Errorf("error")
			rp.Message = "Already executed"
			a.SendRespond(id, rp)
			return
		}
	}

	if a.Cmd == nil {
		a.Cmd = make(map[string]*cmd.Cmd)
	}

	log.Debug().Str("source", "EXEC").Str("action", p).Msg("startExecMqttByID: Start Exec")
	// <-- For Ingest capture only -- //
	src, err := regexp.MatchString(`^(mltcap|mltbackup|maincap|backupcap|archcap)$`, common.EP)
	if err != nil {
		rp.Error = err
		log.Error().Str("source", "EXEC").Err(rp.Error).Msg("startExecMqttByID: regexp failed")
		rp.Message = "Internal error"
		a.SendRespond(id, rp)
	}

	if src == true {
		var ID string
		cs := wf.GetState()
		u, _ := json.Marshal(cs)
		log.Debug().Str("source", "EXEC").RawJSON("json", u).Msg("startExecMqttByID: GetState")
		if common.EP == "mltcap" || common.EP == "maincap" || common.EP == "archcap" {
			ID = cs.CaptureID
		}
		if common.EP == "mltbackup" || common.EP == "backupcap" {
			ID = cs.BackupID
		}
		if cs.CaptureID == "" {
			rp.Error = fmt.Errorf("error")
			log.Error().Str("source", "EXEC").Err(rp.Error).Msg("startExecMqttByID: CaptureID is empty")
			rp.Message = "Internal error"
			a.SendRespond(id, rp)
			//TODO: generate id and start capture
			return
		}

		// Set capture filename with workflow ID
		for k, v := range args {
			switch v {
			case "comment=ID":
				args[k] = strings.Replace(args[k], "ID", ID, 1)
			case "/capture/NAME.mp4":
				args[k] = strings.Replace(args[k], "NAME", ID, 1)
			case "/opt/backup/NAME.mp4":
				args[k] = "/opt/backup/" + ID + ".mp4"

			}
		}
	}
	// -- For Ingest capture only --> //

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
		rp.Error = fmt.Errorf("error")
		rp.Message = "Run Exec Failed"
		a.SendRespond(id, rp)
		return
	}

	rp.Message = "Success"
	a.SendRespond(id, rp)
}

func (a *App) stopExecMqttByID(p string, id string) {

	rp := &MqttPayload{Action: p}
	if a.Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Nothing to stop"
		a.SendRespond(id, rp)
		return
	}

	err := a.Cmd[id].Stop()
	if err != nil {
		rp.Error = err
		rp.Message = "Cmd stop failed"
		a.SendRespond(id, rp)
		return
	}

	removeProgress("stat_" + id + ".log")

	rp.Message = "Success"
	a.SendRespond(id, rp)
}

func (a *App) cmdStatMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	if a.Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		a.SendRespond(id, rp)
		return
	}

	status := a.Cmd[id].Status()

	rp.Message = "Success"
	rp.Data = status
	a.SendRespond(id, rp)

}

func (a *App) execStatusMqttByID(p string, id string) {

	st := make(map[string]interface{})
	rp := &MqttPayload{Action: p}

	if a.Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		a.SendRespond(id, rp)
		return
	}

	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(id, rp)
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

	status := a.Cmd[id].Status()
	isruning, err := PidExists(status.PID)
	if err != nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		a.SendRespond(id, rp)
		return
	}
	st["alive"] = isruning

	//if st["name"] == "ffmpeg" && isruning {
	//	progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
	//	p := <-progress.Start()
	//	report := strings.Split(p.Stdout[0], "\r")
	//	n := len(report)
	//	st["log"] = report[n-1]
	//}

	st["runtime"] = status.Runtime
	st["start"] = status.StartTs
	st["stop"] = status.StopTs

	rp.Message = "Success"
	rp.Data = st
	a.SendRespond(id, rp)
}

func (a *App) execStatusMqtt(p string) {

	var id string
	var services []map[string]interface{}
	rp := &MqttPayload{Action: p}

	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond("false", rp)
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
			status := a.Cmd[id].Status()
			isruning, err := PidExists(status.PID)
			if err != nil {
				rp.Error = fmt.Errorf("error")
				rp.Message = "Cmd not found"
				a.SendRespond(id, rp)
				return
			}

			//if isruning && st["name"] == "ffmpeg" {
			//	progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
			//	p := <-progress.Start()
			//	report := strings.Split(p.Stdout[0], "\r")
			//	n := len(report)
			//	st["log"] = report[n-1]
			//}

			st["alive"] = isruning
			st["runtime"] = status.Runtime
			st["start"] = status.StartTs
			st["stop"] = status.StopTs
		}
		services = append(services, st)
	}

	rp.Message = "Success"
	rp.Data = services
	a.SendRespond(id, rp)
}

func (a *App) getProgressMqtt(p string, id string) {
	rp := &MqttPayload{Action: p}

	if a.Cmd[id] == nil {
		rp.Message = "Not running"
		removeProgress("stat_" + id + ".log")
		return
	}

	status := a.Cmd[id].Status()
	isruning, err := PidExists(status.PID)
	if err != nil {
		rp.Message = "Not running"
		removeProgress("stat_" + id + ".log")
		return
	}

	if isruning {
		progress := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
		pg := <-progress.Start()

		ffjson := make(map[string]string)

		for _, line := range pg.Stdout {
			args := strings.Split(line, "=")
			ffjson[args[0]] = args[1]
		}

		rp.Message = "Success"
		rp.Data = ffjson
	}

	a.SendRespond(id, rp)
}

func (a *App) getReportMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	progress := cmd.NewCmd("cat", "report_"+id+".log")
	pg := <-progress.Start()

	rp.Message = "Success"
	rp.Data = pg
	a.SendRespond(id, rp)
}
