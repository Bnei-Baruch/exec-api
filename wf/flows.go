package wf

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type MqttJson struct {
	Action  string      `json:"action,omitempty"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	Source  string      `json:"src,omitempty"`
	Error   error       `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Result  string      `json:"result,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func MqttMessage(c mqtt.Client, m mqtt.Message) {
	log.Debugf("[MqttMessage] Topic: %s | Message: %s", m.Topic(), m.Payload())
	id := "false"
	s := strings.Split(m.Topic(), "/")

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "workflow" && len(s) == 4 {
		id = s[3]
	}

	mp := &MqttJson{}
	err := json.Unmarshal(m.Payload(), &mp)
	if err != nil {
		log.Errorf("[MqttMessage]: Error Unmarshal: %s", err)
	}

	if id != "false" {
		switch mp.Action {
		case "start":
			go StartFlow(mp, c)
		case "line":
			go LineFlow(mp, c)
		case "stop":
			go StopFlow(mp, c)
		}
	}
}

func SendMessage(topic string, m *MqttJson, c mqtt.Client) {
	message, err := json.Marshal(m)
	if err != nil {
		log.Errorf("[SendMessage]: Message parsing: %s", err)
	}

	if token := c.Publish(topic, byte(1), false, message); token.Wait() && token.Error() != nil {
		log.Errorf("[SendMessage]: Error publish: %s | topic: %s", err, topic)
	}

	log.Debugf("[SendMessage] Topic: %s | Message: %s", topic, string(message))
}

func StartFlow(rp *MqttJson, c mqtt.Client) {

	src := viper.GetString("mqtt.client_id")
	ep := "/ingest/"

	if src == "archcap" {
		ep = "/capture/"
	}

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		log.Errorf("[StartFlow]: CaptureID is empty")
		rp.Message = "Internal error"
		SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
		return
	}

	cm := &MdbPayload{
		CaptureSource: src,
		Station:       GetStationID(src),
		User:          "operator@dev.com",
		FileName:      cs.StartName,
		WorkflowID:    rp.ID,
	}

	err := cm.PostMDB("capture_start")
	if err != nil {
		log.Errorf("[StartFlow]: Post to MDB error: %s", err)
		rp.Error = err
		rp.Message = "MDB Request Failed"
		SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
		return
	}

	ws := &Wfstatus{Capwf: false, Trimmed: false, Sirtutim: false}
	cw := &WfdbCapture{
		CaptureID: rp.ID,
		Date:      GetDateFromID(rp.ID),
		StartName: cs.StartName,
		CapSrc:    src,
		Wfstatus:  *ws,
	}

	err = cw.PutWFDB(rp.Action, ep)
	if err != nil {
		log.Errorf("[StartFlow]: Post to WFDB error: %s", err)
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
		return
	}

	rp.Message = "Success"
	SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
}

func LineFlow(rp *MqttJson, c mqtt.Client) {

	src := viper.GetString("mqtt.client_id")
	ep := "/ingest/"

	if src == "archcap" {
		ep = "/capture/"
	}

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		log.Errorf("[LineFlow]: CaptureID is empty error")
		rp.Message = "Internal error"
		SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
		return
	}

	ws := &Wfstatus{Capwf: false, Trimmed: false, Sirtutim: false}
	cw := &WfdbCapture{
		CaptureID: rp.ID,
		Date:      GetDateFromID(rp.ID),
		StartName: cs.StartName,
		CapSrc:    src,
		Wfstatus:  *ws,
		Line:      cs.Line,
	}

	err := cw.PostWFDB(rp.Action, ep, "line")
	if err != nil {
		log.Errorf("[LineFlow]: Post to WFDB error: %s", err)
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
		return
	}

	rp.Message = "Success"
	SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
}

func StopFlow(rp *MqttJson, c mqtt.Client) {

	src := viper.GetString("mqtt.client_id")
	ep := "/ingest/"

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		log.Errorf("[StopFlow]: CaptureID is empty")
		rp.Message = "Internal error"
		SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
		return
	}

	StopName := cs.StopName
	if src == "archcap" {
		StopName = strings.Replace(StopName, "_o_", "_s_", 1)
	}

	file, err := os.Open(viper.GetString("workflow.capture_path") + rp.ID + ".mp4")
	if err != nil {
		log.Errorf("[StopFlow]: Open file error: %s", err)
		return
	}

	stat, err := file.Stat()
	if err != nil {
		log.Errorf("[StopFlow]: Get stat file error: %s", err)
		return
	}

	size := stat.Size()
	log.Debugf("[StopFlow] File size: %s", size)

	time := stat.Sys().(*syscall.Stat_t)
	//FIXME: WTF?
	ctime := time.Ctimespec.Nsec //OSX
	//ctime := time.Ctim.Nsec //Linux
	log.Debugf("[StopFlow] Creation time file: %s", ctime)

	h := sha1.New()
	if _, err = io.Copy(h, file); err != nil {
		log.Errorf("[StopFlow]: Get shaq file error: %s", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	log.Debugf("[StopFlow] Shaq file: %s", sha)

	cm := &MdbPayload{
		CaptureSource: src,
		Station:       GetStationID(src),
		User:          "operator@dev.com",
		FileName:      StopName,
		WorkflowID:    rp.ID,
		CreatedAt:     ctime,
		Size:          size,
		Sha:           sha,
		Part:          "false",
	}

	cw := &WfdbCapture{}
	err = cw.GetWFDB(rp.ID)
	if err != nil {
		log.Errorf("[StopFlow]: Get WFDB error: %s", err)
		return
	}

	cw.Sha1 = sha
	cw.StopName = StopName

	//Main Multi Capture
	if src == "mltcap" || src == "maincap" {
		if cw.Line.ContentType == "LESSON_PART" {
			cm.Part = strconv.Itoa(cw.Line.Part)
			cm.LessonID = cw.Line.LessonID
			err = cw.PostWFDB(rp.Action, ep, "line")
			if err != nil {
				log.Errorf("[StopFlow]: Post to WFDB error: %s", err)
				return
			}
		}
	}

	//Archive Source Capture
	if src == "archcap" {
		ep = "/capture/"
		cw.CapSrc = "archcap"
		if cw.Line.ContentType == "LESSON_PART" {
			cm.Part = strconv.Itoa(cw.Line.Part)
			cm.LessonID = cw.Line.LessonID
			err = cw.PostWFDB(rp.Action, ep, "line")
			if err != nil {
				log.Errorf("[StopFlow]: Post to WFDB error: %s", err)
				return
			}
		}
	}

	//Backup Multi Capture
	if src == "mltbackup" || src == "backupcap" {
		if cw.Line.ContentType == "LESSON_PART" {
			StopName = StopName[:len(StopName)-2] + "full"
			cw.Line.ContentType = "FULL_LESSON"
			cw.Line.Part = -1
			cw.Line.FinalName = StopName
			cw.StopName = StopName
			cm.Part = "full"
			cm.LessonID = cw.Line.LessonID
			err = cw.PostWFDB(rp.Action, ep, "line")
			if err != nil {
				log.Errorf("[StopFlow]: Post to WFDB error: %s", err)
				return
			}
		}
	}

	err = cw.UpdateWFDB(ep, "sha1?value="+sha)
	if err != nil {
		log.Errorf("[StopFlow]: Update WFDB error: %s", err)
		return
	}

	err = cw.UpdateWFDB(ep, "stop_name?value="+sha)
	if err != nil {
		log.Errorf("[StopFlow]: Update WFDB error: %s", err)
		return
	}

	err = cm.PostMDB("capture_stop")
	if err != nil {
		log.Errorf("[StopFlow]: Post to MDB error: %s", err)
		return
	}

	FullName := StopName + "_" + rp.ID + ".mp4"
	err = os.Rename(viper.GetString("workflow.capture_path")+rp.ID+".mp4", viper.GetString("workflow.capture_path")+FullName)
	if err != nil {
		log.Errorf("[StopFlow]: Rename file error: %s", err)
		return
	}

	cf := CaptureFlow{
		FileName:  FullName,
		Source:    "ingest",
		CapSrc:    src,
		CaptureID: rp.ID,
		Size:      size,
		Sha:       sha,
		Url:       "http://" + cm.Station + ":8080/get/" + FullName,
	}

	err = cf.PutFlow()
	if err != nil {
		log.Errorf("[StopFlow]: Put flow error: %s", err)
		return
	}

	rp.Message = "Success"
	SendMessage(viper.GetString("mqtt.wf_data_topic")+rp.Action, rp, c)
}
