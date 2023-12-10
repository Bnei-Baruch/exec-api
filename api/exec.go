package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/pkg/pgutil"
	"github.com/Bnei-Baruch/exec-api/pkg/wf"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-cmd/cmd"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var Cmd map[string]*cmd.Cmd

func isAliveMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := pgutil.PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Error"
			SendRespond(id, rp)
			return
		}
		if isruning {
			rp.Message = "Alive"
			SendRespond(id, rp)
			return
		}
	}

	rp.Message = "Died"
	SendRespond(id, rp)
}

func startExecMqtt(p string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(viper.GetString("mqtt.client_id"))
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			SendRespond("false", rp)
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
			isruning, err := pgutil.PidExists(status.PID)
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
	SendRespond("false", rp)
}

func stopExecMqtt(p string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(viper.GetString("mqtt.client_id"))
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			SendRespond("false", rp)
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
	SendRespond("false", rp)
}

func startExecMqttByID(p string, id string) {

	rp := &MqttPayload{Action: p}
	c, err := getConf()
	if err != nil {
		c, err = getJson(viper.GetString("mqtt.client_id"))
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			SendRespond(id, rp)
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
		SendRespond(id, rp)
		return
	}

	if Cmd[id] != nil {
		status := Cmd[id].Status()
		isruning, err := pgutil.PidExists(status.PID)
		if err != nil {
			rp.Error = err
			rp.Message = "Internal error"
			SendRespond(id, rp)
			return
		}
		if isruning {
			rp.Error = fmt.Errorf("error")
			rp.Message = "Already executed"
			SendRespond(id, rp)
			return
		}
	}

	if Cmd == nil {
		Cmd = make(map[string]*cmd.Cmd)
	}

	log.Debug().Str("source", "EXEC").Str("action", p).Msg("startExecMqttByID: Start Exec")
	// <-- For Ingest capture only -- //
	src, err := regexp.MatchString(`^(mltcap|mltbackup|maincap|backupcap|archcap|testmaincap|livecap1|livecap2)$`, viper.GetString("mqtt.client_id"))
	if err != nil {
		rp.Error = err
		log.Error().Str("source", "EXEC").Err(rp.Error).Msg("startExecMqttByID: regexp failed")
		rp.Message = "Internal error"
		SendRespond(id, rp)
	}

	cs := wf.GetState()

	if src == true {
		var ID string
		u, _ := json.Marshal(cs)
		log.Debug().Str("source", "EXEC").RawJSON("json", u).Msg("startExecMqttByID: GetState")
		if viper.GetString("mqtt.client_id") == "mltcap" || viper.GetString("mqtt.client_id") == "maincap" || viper.GetString("mqtt.client_id") == "archcap" || viper.GetString("mqtt.client_id") == "testmaincap" || viper.GetString("mqtt.client_id") == "livecap1" || viper.GetString("mqtt.client_id") == "livecap2" {
			ID = cs.CaptureID
		}
		if viper.GetString("mqtt.client_id") == "mltbackup" || viper.GetString("mqtt.client_id") == "backupcap" {
			ID = cs.BackupID
		}
		if cs.CaptureID == "" {
			cs.CaptureID = "CaptureID"
			//rp.Error = fmt.Errorf("error")
			//log.Error().Str("source", "EXEC").Err(rp.Error).Msg("startExecMqttByID: CaptureID is empty")
			//rp.Message = "Internal error"
			//SendRespond(id, rp)
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
			case "/Volumes/FILES/NAME.mp4":
				args[k] = "/Volumes/FILES/" + ID + ".mp4"
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
		SendRespond(id, rp)
		return
	}

	log.Debug().Str("source", "CAP").Msg("start exec")

	rp.Message = "Success"
	SendRespond(id, rp)
	SendState(viper.GetString("mqtt.exec_state_topic"), "On")
}

func stopExecMqttByID(p string, id string) {

	rp := &MqttPayload{Action: p}
	cs := wf.GetState()

	if Cmd[id] == nil {
		pid := pgutil.GetPID()
		if pid > 0 {
			syscall.Kill(pid, syscall.SIGTERM)
			cs.IsRec = false
			removeProgress("stat_" + id + ".log")
			rp.Message = "Success"
		} else {
			rp.Error = fmt.Errorf("error")
			rp.Message = "Nothing to stop"
		}
		SendRespond(id, rp)
		SendState(viper.GetString("mqtt.exec_state_topic"), "Off")
		return
	}

	err := Cmd[id].Stop()
	if err != nil {
		rp.Error = err
		rp.Message = "Cmd stop failed"
		SendRespond(id, rp)
		return
	}

	removeProgress("stat_" + id + ".log")

	rp.Message = "Success"
	SendRespond(id, rp)
	SendState(viper.GetString("mqtt.exec_state_topic"), "Off")
}

func cmdStatMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	if Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		SendRespond(id, rp)
		return
	}

	status := Cmd[id].Status()

	rp.Message = "Success"
	rp.Data = status
	SendRespond(id, rp)

}

func execStatusMqttByID(p string, id string) {

	st := make(map[string]interface{})
	rp := &MqttPayload{Action: p}

	if Cmd[id] == nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		SendRespond(id, rp)
		return
	}

	c, err := getConf()
	if err != nil {
		c, err = getJson(viper.GetString("mqtt.client_id"))
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			SendRespond(id, rp)
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
	isruning, err := pgutil.PidExists(status.PID)
	if err != nil {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Cmd not found"
		SendRespond(id, rp)
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
	SendRespond(id, rp)
}

func execStatusMqtt(p string) {

	var id string
	var services []map[string]interface{}
	rp := &MqttPayload{Action: p}

	c, err := getConf()
	if err != nil {
		c, err = getJson(viper.GetString("mqtt.client_id"))
		if err != nil {
			rp.Error = err
			rp.Message = "Failed get config"
			SendRespond("false", rp)
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
			isruning, err := pgutil.PidExists(status.PID)
			if err != nil {
				rp.Error = fmt.Errorf("error")
				rp.Message = "Cmd not found"
				SendRespond(id, rp)
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
	SendRespond(id, rp)
}

func getProgressMqtt(p string, id string) {

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
	SendRespond(id, rp)
}

func getReportMqtt(p string, id string) {

	rp := &MqttPayload{Action: p}
	progress := cmd.NewCmd("cat", "report_"+id+".log")
	pg := <-progress.Start()

	rp.Message = "Success"
	rp.Data = pg
	SendRespond(id, rp)
}

var Ticker *time.Ticker
var tick bool

func SendProgress(on bool) {
	log.Debug().Str("source", "CAP").Msg("SendProgress")
	ExecDataTopic := viper.GetString("mqtt.exec_data_topic")
	ClientID := viper.GetString("mqtt.client_id")
	rp := &MqttPayload{Action: "progress"}
	if on && !tick {
		Ticker = time.NewTicker(1000 * time.Millisecond)
		tick = true
		go func() {
			for range Ticker.C {
				pid := pgutil.GetPID()
				if pid == 0 {
					Ticker.Stop()
					rp.Message = "Off"
				} else {
					progress := cmd.NewCmd("tail", "-n", "12", "stat_sdi"+".log")
					pg := <-progress.Start()

					if pg.Error != nil {
						return
					}

					ffjson := make(map[string]string)

					for _, line := range pg.Stdout {
						args := strings.Split(line, "=")
						ffjson[args[0]] = args[1]
					}

					rp.Message = "On"
					rp.Data = ffjson
				}
				SendMessage(ExecDataTopic+ClientID, rp)
			}
		}()
	}

	if !on && tick {
		tick = false
		Ticker.Stop()
		rp.Message = "Off"
		SendMessage(ExecDataTopic+ClientID, rp)
	}
}

func ExecState(c mqtt.Client, m mqtt.Message) {
	log.Debug().Str("source", "MQTT").Str("json", string(m.Payload())).Msg("Got Exec State: " + string(m.Payload()))
	src, _ := regexp.MatchString(`^(mltcap|mltbackup|maincap|backupcap|archcap|testmaincap)$`, viper.GetString("mqtt.client_id"))
	if src {
		data := string(m.Payload())
		if data == "On" {
			pid := pgutil.GetPID()
			if pid != 0 {
				SendProgress(true)
				return
			}
		}
		if data == "Off" {
			SendProgress(false)
		}
	}
}
