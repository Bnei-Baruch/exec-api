package api

import (
	"encoding/json"
	"fmt"
	"github.com/go-cmd/cmd"
	"os"
	"strings"
	"time"
)

func (a *App) isAliveMqtt(ep string, id string) {

	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Error"}`)
			return
		}
		if isruning {
			a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Alive"}`)
			return
		}
	}

	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Died"}`)
}

func (a *App) startExecMqtt(ep string) {

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			a.Publish("exec/service/data/"+ep, `{"error": 1, "message":"Failed get config"}`)
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
	a.Publish("exec/service/data/"+ep, `{"error": 0, "message":"Success"}`)
}

func (a *App) stopExecMqtt(ep string) {

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			a.Publish("exec/service/data/"+ep, `{"error": 1, "message":"Failed get config"}`)
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
	a.Publish("exec/service/data/"+ep, `{"error": 0, "message":"Success"}`)
}

func (a *App) startExecMqttByID(ep string, id string) {

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			a.Publish("exec/service/data/"+ep, `{"error": 1, "message":"Failed get config"}`)
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
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"ID not found"}`)
		return
	}

	if a.Cmd[id] != nil {
		status := a.Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Error"}`)
			return
		}
		if isruning {
			a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Already executed"}`)
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
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Run Exec Failed"}`)
		return
	}

	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Success"}`)
}

func (a *App) stopExecMqttByID(ep string, id string) {

	if a.Cmd[id] == nil {
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Nothing to stop"}`)
		return
	}

	err := a.Cmd[id].Stop()
	if err != nil {
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Error"}`)
		return
	}

	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Success"}`)
}

func (a *App) cmdStatMqtt(ep string, id string) {

	if a.Cmd[id] == nil {
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Nothing to show"}`)
		return
	}

	status := a.Cmd[id].Status()
	data, _ := json.Marshal(status)

	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Success", "data": `+string(data)+`}`)

}

func (a *App) execStatusMqttByID(ep string, id string) {

	st := make(map[string]interface{})

	if a.Cmd[id] == nil {
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Nothing to show"}`)
		return
	}

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			a.Publish("exec/service/data/"+ep, `{"error": 1, "message":"Failed get config"}`)
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
		a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 1, "message":"Error"}`)
		return
	}
	st["alive"] = isruning

	if st["name"] == "ffmpeg" && isruning {
		progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
		p := <-progress.Start()
		report := strings.Split(p.Stdout[0], "\r")
		n := len(report)
		st["log"] = report[n-1]
	}

	//prog := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	//pr := <-prog.Start()
	//for _, line := range pr.Stdout {
	//	args := strings.Split(line, "=")
	//	st[args[0]] = args[1]
	//}

	st["runtime"] = status.Runtime
	st["start"] = status.StartTs
	st["stop"] = status.StopTs

	data, _ := json.Marshal(st)

	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Success", "data": `+string(data)+`}`)
}

func (a *App) execStatusMqtt(ep string) {

	var id string
	var services []map[string]interface{}

	c, err := getConf()
	if err != nil {
		c, err = getJson(ep)
		if err != nil {
			a.Publish("exec/service/data/"+ep, `{"error": 1, "message":"Failed get config"}`)
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
				a.Publish("exec/service/data/"+ep, `{"error": 1, "message":"Error"}`)
				return
			}

			if isruning && st["name"] == "ffmpeg" {
				progress := cmd.NewCmd("tail", "-c", "1000", "report_"+id+".log")
				p := <-progress.Start()
				report := strings.Split(p.Stdout[0], "\r")
				n := len(report)
				st["log"] = report[n-1]
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

	data, err := json.Marshal(services)

	if err != nil {
		fmt.Printf("Received message: %s from topic: %s\n", err, string(data))
	}

	a.Publish("exec/service/data/"+ep, `{"error": 0, "message":"Success", "data": `+string(data)+`}`)
}

func (a *App) getProgressMqtt(ep string, id string) {

	progress := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	p := <-progress.Start()

	ffjson := make(map[string]string)

	for _, line := range p.Stdout {
		args := strings.Split(line, "=")
		ffjson[args[0]] = args[1]
	}

	data, _ := json.Marshal(ffjson)

	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Success", "data": `+string(data)+`}`)
}

func (a *App) getReportMqtt(ep string, id string) {

	progress := cmd.NewCmd("cat", "report_"+id+".log")
	p := <-progress.Start()

	data, _ := json.Marshal(p)
	a.Publish("exec/service/data/"+ep+"/"+id, `{"error": 0, "message":"Success", "data": `+string(data)+`}`)

}
