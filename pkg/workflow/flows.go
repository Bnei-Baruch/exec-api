package workflow

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"io"
	"os"
	"strconv"
	"strings"
)

type MqttWorkflow struct {
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
	//fmt.Printf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	id := "false"
	s := strings.Split(m.Topic(), "/")
	ep := os.Getenv("MQTT_EP")

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "workflow" && len(s) == 4 {
		id = s[3]
	}

	mp := MqttWorkflow{}
	err := json.Unmarshal(m.Payload(), &mp)
	if err != nil {
		fmt.Println(err.Error())
	}

	if id != "false" {
		switch mp.Action {
		case "start":
			go StartFlow(mp, ep, c)
		case "stop":
			go StopFlow(mp, ep, c)
		}
	}
}

func Publish(topic string, message string, c mqtt.Client) {
	text := fmt.Sprintf(message)
	//a.Msg.Publish(topic, byte(1), false, text)
	if token := c.Publish(topic, byte(1), false, text); token.Wait() && token.Error() != nil {
		fmt.Printf("Publish message error: %s\n", token.Error())
	}
}

func StartFlow(rp MqttWorkflow, src string, c mqtt.Client) {

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Internal error"
		m, _ := json.Marshal(rp)
		Publish("workflow/service/data/"+rp.Action, string(m), c)
		return
	}

	cm := &MdbPayload{
		CaptureSource: src,
		Station:       GetStationID(src),
		User:          "operator@dev.com",
		FileName:      cs.StartName,
		WorkflowID:    cs.CaptureID,
	}

	err := cm.PostMDB("capture_start")
	if err != nil {
		rp.Error = err
		rp.Message = "MDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish("workflow/service/data/"+rp.Action, string(m), c)
		return
	}

	ws := &Wfstatus{Capwf: false, Trimmed: false, Sirtutim: false}
	cw := &WfdbCapture{
		CaptureID: cs.CaptureID,
		Date:      GetDateFromID(cs.CaptureID),
		StartName: cs.StartName,
		CapSrc:    src,
		Wfstatus:  *ws,
	}

	err = cw.PutWFDB()
	if err != nil {
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish("workflow/service/data/"+rp.Action, string(m), c)
		return
	}

	rp.Message = "Success"
	m, _ := json.Marshal(rp)

	Publish("workflow/service/data/"+rp.Action, string(m), c)
}

func StopFlow(rp MqttWorkflow, src string, c mqtt.Client) {

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		rp.Message = "Internal error"
		m, _ := json.Marshal(rp)
		Publish("workflow/service/data/"+rp.Action, string(m), c)
		return
	}

	StopName := cs.StopName

	file, err := os.Open("/capture/" + cs.CaptureID + ".mp4")
	if err != nil {
		return
	}

	stat, err := file.Stat()
	if err != nil {
		return
	}

	size := stat.Size()
	fmt.Println("stopFlow file size:", size)

	h := sha1.New()
	if _, err = io.Copy(h, file); err != nil {
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Println("stopFlow file sha1:", sha)

	cm := &MdbPayload{
		CaptureSource: src,
		Station:       GetStationID(src),
		User:          "operator@dev.com",
		FileName:      StopName,
		WorkflowID:    cs.CaptureID,
		Size:          size,
		Sha:           sha,
		Part:          "false",
	}

	cw := &WfdbCapture{}
	err = cw.GetWFDB(cs.CaptureID)
	if err != nil {
		fmt.Println("WfdbRead Failed:", err)
		return
	}

	cw.Sha1 = sha
	cw.StopName = StopName

	//Main Multi Capture
	if src == "mltmain" || src == "maincap" {
		if cw.Line.ContentType == "LESSON_PART" {
			cm.Part = strconv.Itoa(cw.Line.Part)
			cm.LessonID = cw.Line.LessonID
		}
	}

	//Backup Multi Capture
	if src == "mltbackup" || src == "backupcap" {
		cw.Line = cs.Line
		if cw.Line.ContentType == "LESSON_PART" {
			StopName := StopName[:len(StopName)-2] + "full"
			cw.Line.ContentType = "FULL_LESSON"
			cw.Line.Part = -1
			cw.Line.LessonID = cs.Line.LessonID
			cw.Line.FinalName = StopName
			cw.StopName = StopName
			cm.Part = "full"
			cm.LessonID = cw.Line.LessonID
		}
	}

	err = cw.PutWFDB()
	if err != nil {
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish("workflow/service/data/"+rp.Action, string(m), c)
		return
	}

	err = cm.PostMDB("capture_stop")
	if err != nil {
		return
	}

	FullName := StopName + "_" + cs.CaptureID + ".mp4"
	err = os.Rename("/capture/"+cs.CaptureID+".mp4", "/capture/"+FullName)
	if err != nil {
		return
	}

	cf := CaptureFlow{
		FileName:  FullName,
		Source:    "ingest",
		CapSrc:    rp.Source,
		CaptureID: cs.CaptureID,
		Size:      size,
		Sha:       sha,
		Url:       "http://" + cm.Station + ":8081/get/" + FullName,
	}

	err = cf.PutFlow()
	if err != nil {
		return
	}

	rp.Message = "Success"
	m, _ := json.Marshal(rp)

	Publish("workflow/service/data/"+rp.Action, string(m), c)
}
