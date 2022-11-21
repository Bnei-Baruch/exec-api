package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"github.com/Bnei-Baruch/exec-api/pkg/wf"
	"github.com/go-cmd/cmd"
	"github.com/rs/zerolog/log"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var Cmd map[string]*cmd.Cmd

func (m *Mqtt) isAliveMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Error"
			m.SendRespond(id, rp)
			return
		}
		if isruning {
			rp.Message = "Alive"
			m.SendRespond(id, rp)
			return
		}
	}

	rp.Message = "Died"
	m.SendRespond(id, rp)
}

func (m *Mqtt) startExecMqtt(p string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			m.SendRespond("false", rp)
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

		if Cmd[id] != nil {
			status := Cmd[id].Status()
			isruning, err := PidExists(status.PID)
			if err != nil {
				continue
			}
			if isruning {
				continue
			}
		}

		if Cmd == nil {
			Cmd = make(map[string]*cmd.Cmd)
		}

		if service == "ffmpeg" {
			cmdOptions := cmd.Options{Buffered: false, Streaming: false}
			os.Setenv("FFREPORT", "file=report_"+id+".log:level=32")
			Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
		} else {
			Cmd[id] = cmd.NewCmd(service, args...)
		}

		Cmd[id].Start()

		time.Sleep(2 * time.Second)

		status := Cmd[id].Status()

		if status.Exit == 1 {
			continue
		}
	}

	// TODO: return exec report
	rp.Message = "Success"
	m.SendRespond("false", rp)
}

func (m *Mqtt) stopExecMqtt(p string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			m.SendRespond("false", rp)
			return
		}
	}

	var id string
	for _, v := range c.Services {
		id = v.ID

		if Cmd[id] == nil {
			continue
		}

		err := Cmd[id].Stop()
		if err != nil {
			continue
		}
	}

	// TODO: return report
	rp.Message = "Success"
	m.SendRespond("false", rp)
}

func (m *Mqtt) startExecMqttByID(p string, id string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			m.SendRespond(id, rp)
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
		m.SendRespond(id, rp)
		return
	}

	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Internal error"
			m.SendRespond(id, rp)
			return
		}
		if isruning {
			rp.Error = fmt.Errorf("error")
			rp.Message = "Already executed"
			m.SendRespond(id, rp)
			return
		}
	}

	if Cmd == nil {
		Cmd = make(map[string]*cmd.Cmd)
	}

	log.Debug().Str("source", "EXEC").Str("action", p).Msg("startExecMqttByID: Start Exec")
	// <-- For Ingest capture only -- //
	src, err := regexp.MatchString(`^(mltcap|mltbackup|maincap|backupcap|archcap|testcap)$`, common.EP)
	if err != nil {
		rp.Error = err
		log.Error().Str("source", "EXEC").Err(rp.Error).Msg("startExecMqttByID: regexp failed")
		rp.Message = "Internal error"
		m.SendRespond(id, rp)
	}

	if src == true {
		var ID string
		cs := wf.GetState()
		u, _ := json.Marshal(cs)
		log.Debug().Str("source", "EXEC").RawJSON("json", u).Msg("startExecMqttByID: GetState")
		if common.EP == "mltcap" || common.EP == "maincap" || common.EP == "archcap" || common.EP == "testcap" {
			ID = cs.CaptureID
		}
		if common.EP == "mltbackup" || common.EP == "backupcap" {
			ID = cs.BackupID
		}
		if cs.CaptureID == "" {
			cs.CaptureID = "CaptureID"
			//rp.Error = fmt.Errorf("error")
			//log.Error().Str("source", "EXEC").Err(rp.Error).Msg("startExecMqttByID: CaptureID is empty")
			//rp.Message = "Internal error"
			//m.SendRespond(id, rp)
			////TODO: generate id and start capture
			//return
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
		Cmd[id] = cmd.NewCmdOptions(cmdOptions, service, args...)
	} else {
		Cmd[id] = cmd.NewCmd(service, args...)
	}

	Cmd[id].Start()
	status := Cmd[id].Status()
	if status.Exit == 1 {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Run Exec Failed"
		m.SendRespond(id, rp)
		return
	}

	rp.Message = "Success"
	m.SendRespond(id, rp)
}

func (m *Mqtt) stopExecMqttByID(p string, id string) {

	rp := &MqttPayload{Action: p}
	if Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Nothing to stop"
		m.SendRespond(id, rp)
		syscall.Kill(GetPID(), syscall.SIGTERM)
		return
	}

	err := Cmd[id].Stop()
	if err != nil {
		rp.Error = err
		rp.Message = "Cmd stop failed"
		m.SendRespond(id, rp)
		return
	}

	removeProgress("stat_" + id + ".log")

	rp.Message = "Success"
	m.SendRespond(id, rp)
}

func (m *Mqtt) cmdStatMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	if Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		m.SendRespond(id, rp)
		return
	}

	status := Cmd[id].Status()

	rp.Message = "Success"
	rp.Data = status
	m.SendRespond(id, rp)

}

func (m *Mqtt) execStatusMqttByID(p string, id string) {

	st := make(map[string]interface{})
	rp := &MqttPayload{Action: p}

	if Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		m.SendRespond(id, rp)
		return
	}

	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			m.SendRespond(id, rp)
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

	status := Cmd[id].Status()
	isruning, err := PidExists(status.PID)
	if err != nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		m.SendRespond(id, rp)
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
	m.SendRespond(id, rp)
}

func (m *Mqtt) execStatusMqtt(p string) {

	var id string
	var services []map[string]interface{}
	rp := &MqttPayload{Action: p}

	c, err := getConf()
	if err != nil {
		c, err = getJson(common.EP)
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			m.SendRespond("false", rp)
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

		if Cmd[id] == nil {
			st["runtime"] = 0
			st["start"] = 0
			st["stop"] = 0
			st["alive"] = false
		} else {
			status := Cmd[id].Status()
			isruning, err := PidExists(status.PID)
			if err != nil {
				rp.Error = fmt.Errorf("error")
				rp.Message = "Cmd not found"
				m.SendRespond(id, rp)
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
	m.SendRespond(id, rp)
}

func (m *Mqtt) getProgressMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	progress := cmd.NewCmd("tail", "-n", "12", "stat_"+id+".log")
	pg := <-progress.Start()

	ffjson := make(map[string]string)

	for _, line := range pg.Stdout {
		args := strings.Split(line, "=")
		ffjson[args[0]] = args[1]
	}

	rp.Message = "Success"
	rp.Data = ffjson
	m.SendRespond(id, rp)
}

func (m *Mqtt) getReportMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	progress := cmd.NewCmd("cat", "report_"+id+".log")
	pg := <-progress.Start()

	rp.Message = "Success"
	rp.Data = pg
	m.SendRespond(id, rp)
}
