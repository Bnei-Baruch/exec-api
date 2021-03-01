package api

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/pkg/workflow"
	"io"
	"os"
	"strconv"
)

func (a *App) startFlow(rp MqttPayload, id string) {

	cm := &workflow.MdbPayload{
		CaptureSource: id,
		Station:       workflow.GetStationID(id),
		User:          "operator@dev.com",
		FileName:      rp.Name,
		WorkflowID:    rp.ID,
	}

	err := cm.PostMDB("capture_start")
	if err != nil {
		rp.Error = err
		rp.Message = "MDB Request Failed"
		m, _ := json.Marshal(rp)
		a.Publish("workflow/service/data/"+rp.Action, string(m))
		return
	}

	ws := &workflow.Wfstatus{Capwf: false, Trimmed: false, Sirtutim: false}
	cw := &workflow.WfdbCapture{
		CaptureID: rp.ID,
		Date:      workflow.GetDateFromID(rp.ID),
		StartName: rp.Name,
		CapSrc:    id,
		Wfstatus:  *ws,
	}

	err = cw.PutWFDB()
	if err != nil {
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		m, _ := json.Marshal(rp)
		a.Publish("workflow/service/data/"+rp.Action, string(m))
		return
	}

	rp.Message = "Success"
	m, _ := json.Marshal(rp)

	a.Publish("workflow/service/data/"+rp.Action, string(m))
}

func (a *App) stopFlow(rp MqttPayload, id string) {

	StopName := rp.Name

	file, err := os.Open("/capture/" + id + ".mp4")
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

	cm := &workflow.MdbPayload{
		CaptureSource: id,
		Station:       workflow.GetStationID(id),
		User:          "operator@dev.com",
		FileName:      StopName,
		WorkflowID:    rp.ID,
		Size:          size,
		Sha:           sha,
		Part:          "false",
	}

	cw := &workflow.WfdbCapture{}
	err = cw.GetWFDB(id)
	if err != nil {
		fmt.Println("WfdbRead Failed:", err)
		return
	}

	cw.Sha1 = sha
	cw.StopName = StopName

	//Main Multi Capture
	if id == "mltmain" {
		if cw.Line.ContentType == "LESSON_PART" {
			cm.Part = strconv.Itoa(cw.Line.Part)
			cm.LessonID = cw.Line.LessonID
		}
	}

	//Backup Multi Capture
	if id == "mltbackup" {
		cs, err := workflow.GetCaptureState(id)
		if err != nil {
			fmt.Println("GetCaptureState Failed:", err)
			return
		}
		cw.Line = cs.Line
		if cw.Line.ContentType == "LESSON_PART" {
			StopName := rp.Name[:len(rp.Name)-2] + "full"
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
		a.Publish("workflow/service/data/"+rp.Action, string(m))
		return
	}

	err = cm.PostMDB("capture_stop")
	if err != nil {
		return
	}

	FullName := StopName + "_" + id + ".mp4"
	err = os.Rename("/capture/"+id+".mp4", "/capture/"+FullName)
	if err != nil {
		return
	}

	cf := workflow.CaptureFlow{
		FileName:  FullName,
		Source:    "ingest",
		CapSrc:    rp.Source,
		CaptureID: id,
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

	a.Publish("workflow/service/data/"+rp.Action, string(m))
}
