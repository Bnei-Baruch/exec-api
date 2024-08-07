package wf

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type MdbPayload struct {
	CaptureSource string `json:"capture_source"`
	Station       string `json:"station"`
	User          string `json:"user"`
	FileName      string `json:"file_name"`
	WorkflowID    string `json:"workflow_id"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	LessonID      string `json:"collection_uid,omitempty"`
	Part          string `json:"part,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Sha           string `json:"sha1,omitempty"`
}

type Wfstatus struct {
	Capwf    bool `json:"capwf"`
	Trimmed  bool `json:"trimmed"`
	Sirtutim bool `json:"sirtutim"`
}

type CaptureState struct {
	Action    string `json:"action"`
	BackupID  string `json:"backup_id"`
	CaptureID string `json:"capture_id"`
	Date      string `json:"date"`
	IsRec     bool   `json:"isRec"`
	IsHag     bool   `json:"isHag"`
	LineID    string `json:"line_id"`
	NextPart  bool   `json:"next_part"`
	ReqDate   string `json:"req_date"`
	StartName string `json:"start_name"`
	StopName  string `json:"stop_name"`
	Line      Line   `json:"line"`
	NumPrt    NumPrt `json:"num_prt"`
}

type NumPrt struct {
	Lesson  int `json:"LESSON_PART"`
	Meal    int `json:"MEAL"`
	Friends int `json:"FRIENDS_GATHERING"`
	Unknown int `json:"UNKNOWN"`
	Part    int `json:"part"`
}

type WfdbCapture struct {
	CaptureID string                 `json:"capture_id"`
	CapSrc    string                 `json:"capture_src"`
	Date      string                 `json:"date"`
	StartName string                 `json:"start_name"`
	StopName  string                 `json:"stop_name"`
	Sha1      string                 `json:"sha1"`
	Line      Line                   `json:"line"`
	Original  map[string]interface{} `json:"original"`
	Proxy     map[string]interface{} `json:"proxy"`
	Wfstatus  Wfstatus               `json:"wfstatus"`
}

type Line struct {
	ArtifactType   string   `json:"artifact_type"`
	AutoName       string   `json:"auto_name"`
	CaptureDate    string   `json:"capture_date"`
	CollectionType string   `json:"collection_type"`
	CollectionUID  string   `json:"collection_uid,omitempty"`
	CollectionID   int      `json:"collection_id,omitempty"`
	ContentType    string   `json:"content_type"`
	Episode        string   `json:"episode,omitempty"`
	FinalName      string   `json:"final_name"`
	FilmDate       string   `json:"film_date,omitempty"`
	HasTranslation bool     `json:"has_translation"`
	Holiday        bool     `json:"holiday"`
	Language       string   `json:"language"`
	Lecturer       string   `json:"lecturer"`
	LessonID       string   `json:"lid"`
	ManualName     string   `json:"manual_name"`
	Number         int      `json:"number"`
	Part           int      `json:"part"`
	PartType       int      `json:"part_type,omitempty"`
	Major          *Major   `json:"major,omitempty"`
	Pattern        string   `json:"pattern"`
	RequireTest    bool     `json:"require_test"`
	Likutim        []string `json:"likutims,omitempty"`
	Sources        []string `json:"sources"`
	Tags           []string `json:"tags"`
}

type Major struct {
	Type string `json:"type" binding:"omitempty,eq=source|eq=tag|eq=likutim"`
	Idx  int    `json:"idx" binding:"omitempty,gte=0"`
}

type CaptureFlow struct {
	FileName  string `json:"file_name"`
	Source    string `json:"source"`
	CapSrc    string `json:"capture_src"`
	CaptureID string `json:"capture_id"`
	Size      int64  `json:"size"`
	Sha       string `json:"sha1"`
	Url       string `json:"url"`
}

var Data []byte

func GetMemState() *CaptureState {
	var cs *CaptureState
	err := json.Unmarshal(Data, &cs)
	if err != nil {
		log.Errorf("[GetMemState]: Error Unmarshal: %s", err)
	}
	u, err := json.Marshal(cs)
	if err != nil {
		log.Errorf("[GetMemState]: Error Marshal: %s", err)
	}
	log.Debugf("[GetMemState] : %s", u)
	return cs
}

func GetState() *CaptureState {
	var cs *CaptureState
	s, err := os.ReadFile("state.json")
	if err != nil {
		log.Errorf("[GetState]: Error read file: %s", err)
	}
	err = json.Unmarshal(s, &cs)
	if err != nil {
		log.Errorf("[GetState]: Error Unmarshal state: %s", err)
	}
	u, _ := json.Marshal(cs)
	log.Debugf("[GetState] : %s", u)
	return cs
}

func SetState(c mqtt.Client, m mqtt.Message) {
	cs := &CaptureState{}
	Data = m.Payload()
	err := json.Unmarshal(m.Payload(), &cs)
	if err != nil {
		log.Errorf("[GetState]: Error Unmarshal state: %s", err)
	}
	u, _ := json.Marshal(cs)
	log.Debugf("[SetState] : %s", u)
	err = os.WriteFile("state.json", u, 0644)
	if err != nil {
		log.Errorf("[SetState]: Error save state: %s", err)
	}

	//pid := pgutil.GetPID()
	//if pid == 0 && cs.IsRec {
	//	cs.IsRec = false
	//	w.SendState(common.StateTopic, cs)
	//	return
	//}
	//
	//w.SendProgress(cs.IsRec)
}

func (m *MdbPayload) PostMDB(ep string) error {
	u, _ := json.Marshal(m)
	log.Debugf("[PostMDB] action: %s | json: %s", ep, u)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("POST", viper.GetString("workflow.mdb_url")+ep, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func (w *WfdbCapture) GetWFDB(id string) error {
	req, err := http.NewRequest("GET", viper.GetString("workflow.wfdb_url")+"/ingest/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	log.Debugf("[GetWFDB] json: %s", body)
	err = json.Unmarshal(body, &w)
	if err != nil {
		return err
	}

	return nil
}

func (w *WfdbCapture) GetIngestState(id string) error {
	req, err := http.NewRequest("GET", viper.GetString("workflow.wfdb_url")+"/state/ingest/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	log.Debugf("[GetIngestState] json: %s", body)
	err = json.Unmarshal(body, &w)
	if err != nil {
		return err
	}

	return nil
}

func (w *WfdbCapture) PutWFDB(action string, ep string) error {
	u, _ := json.Marshal(w)
	log.Debugf("[PutWFDB] action: %s | json: %s", action, u)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", viper.GetString("workflow.wfdb_url")+ep+w.CaptureID, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func (w *WfdbCapture) PostWFDB(action string, ep string, key string) error {
	u, _ := json.Marshal(w.Line)
	log.Debugf("[PostWFDB] action: %s | json: %s", action, u)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("POST", viper.GetString("workflow.wfdb_url")+ep+w.CaptureID+"/"+key, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func (w *WfdbCapture) UpdateWFDB(ep string, value string) error {
	log.Debugf("[UpdateWFDB] action: update | value: %s", value)
	req, err := http.NewRequest("POST", viper.GetString("workflow.wfdb_url")+ep+w.CaptureID+"/"+value, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func (w *CaptureFlow) PutFlow() error {
	u, _ := json.Marshal(w)
	log.Debugf("[PutFlow] json: %s", u)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", viper.GetString("workflow.wfapi_url"), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func GetStationID(id string) string {
	switch id {
	case "mltcap":
		return viper.GetString("workflow.mlt_main")
	case "mltbackup":
		return viper.GetString("workflow.mlt_backup")
	case "maincap":
		return viper.GetString("workflow.main_cap")
	case "backupcap":
		return viper.GetString("workflow.backup_cap")
	case "archcap":
		return viper.GetString("workflow.arch_cap")
	case "archbackup":
		return viper.GetString("workflow.arch_backup")
	}

	return "127.0.0.1"
}

func GetDateFromID(id string) string {
	ts := strings.Split(id, "c")[1]
	tsInt, _ := strconv.ParseInt(ts, 10, 64)
	tsTime := time.Unix(0, tsInt*int64(time.Millisecond))
	return strings.Split(tsTime.String(), " ")[0]
}
