package api

import (
	"fmt"
	"github.com/Bnei-Baruch/exec-api/pkg/workflow"
	"github.com/go-cmd/cmd"
	"os"
	"regexp"
	"strings"
	"time"
)

func (a *App) isAliveMqtt(ep string, id string) {

	rp := &MqttPayload{}
	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Error"
			a.SendRespond(ep, id, rp)
			return
		}
		if isruning {
			rp.Message = "Alive"
			a.SendRespond(ep, id, rp)
			return
		}
	}

	rp.Message = "Died"
	a.SendRespond(ep, id, rp)
}

func (a *App) startExecMqtt(ep string) {

	rp := &MqttPayload{}
	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(ep, "false", rp)
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
	a.SendRespond(ep, "false", rp)
}

func (a *App) stopExecMqtt(ep string) {

	rp := &MqttPayload{}
	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(ep, "false", rp)
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
	rp.Message = "Success"
	a.SendRespond(ep, "false", rp)
}

func (a *App) startExecMqttByID(ep string, id string) {

	rp := &MqttPayload{}
	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(ep, id, rp)
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
		a.SendRespond(ep, id, rp)
		return
	}

	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Internal error"
			a.SendRespond(ep, id, rp)
			return
		}
		if isruning {
			rp.Error = fmt.Errorf("error")
			rp.Message = "Already executed"
			a.SendRespond(ep, id, rp)
			return
		}
	}

	if a.Cmd == nil {
		a.Cmd = make(map[string]*cmd.Cmd)
	}

	// <-- For Ingest capture only -- //
	src, err := regexp.MatchString(`^(mltmain|mltbackup|maincap|backupcap)$`, id)
	if err != nil {
		rp.Error = err
		rp.Message = "Internal error"
		a.SendRespond(ep, id, rp)
	}

	if src == true {
		cs, err := workflow.GetCaptureState(id)
		if err != nil {
			rp.Error = err
			rp.Message = "Internal error"
			a.SendRespond(ep, id, rp)
			//TODO: generate id and start capture
			return
		}

		// Set capture filename with workflow ID
		for k, v := range args {
			switch v {
			case "comment=ID":
				args[k] = strings.Replace(args[k], "ID", cs.CaptureID, 1)
				args[k] = cs.CaptureID
			case "/capture/NAME.mp4":
				args[k] = strings.Replace(args[k], "NAME", cs.CaptureID, 1)
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
		a.SendRespond(ep, id, rp)
		return
	}

	rp.Message = "Success"
	a.SendRespond(ep, id, rp)
}

func (a *App) stopExecMqttByID(ep string, id string) {

	rp := &MqttPayload{}
	if a.Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Nothing to stop"
		a.SendRespond(ep, id, rp)
		return
	}

	err := a.Cmd[id].Stop()
	if err != nil {
		rp.Error = err
		rp.Message = "Cmd stop failed"
		a.SendRespond(ep, id, rp)
		return
	}

	rp.Message = "Success"
	a.SendRespond(ep, id, rp)
}

func (a *App) cmdStatMqtt(ep string, id string) {

	rp := &MqttPayload{}
	if a.Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		a.SendRespond(ep, id, rp)
		return
	}

	status := a.Cmd[id].Status()

	rp.Message = "Success"
	rp.Data = status
	a.SendRespond(ep, id, rp)

}

func (a *App) execStatusMqttByID(ep string, id string) {

	st := make(map[string]interface{})
	rp := &MqttPayload{}

	if a.Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		a.SendRespond(ep, id, rp)
		return
	}

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(ep, id, rp)
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
		a.SendRespond(ep, id, rp)
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
	a.SendRespond(ep, id, rp)
}

func (a *App) execStatusMqtt(ep string) {

	var id string
	var services []map[string]interface{}
	rp := &MqttPayload{}

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			a.SendRespond(ep, "false", rp)
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
				a.SendRespond(ep, id, rp)
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
	a.SendRespond(ep, id, rp)
}

func (a *App) getProgressMqtt(ep string, id string) {

	rp := &MqttPayload{}
	progress := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	p := <-progress.Start()

	ffjson := make(map[string]string)

	for _, line := range p.Stdout {
		args := strings.Split(line, "=")
		ffjson[args[0]] = args[1]
	}

	rp.Message = "Success"
	rp.Data = ffjson
	a.SendRespond(ep, id, rp)
}

func (a *App) getReportMqtt(ep string, id string) {

	rp := &MqttPayload{}
	progress := cmd.NewCmd("cat", "report_"+id+".log")
	p := <-progress.Start()

	rp.Message = "Success"
	rp.Data = p
	a.SendRespond(ep, id, rp)
}
