package workflow

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"io/ioutil"
	"net/http"
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
	CaptureID string `json:"capture_id"`
	Date      string `json:"date"`
	LessonID  string `json:"lid"`
	IsHag     bool   `json:"ishag"`
	LineID    string `json:"lineid"`
	NextPart  bool   `json:"next_part"`
	ReqDate   string `json:"reqdate"`
	StartName string `json:"startname"`
	StopName  string `json:"stopname"`
	Line      Line   `json:"line"`
	NumPrt    NumPrt `json:"numprt"`
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
	ContentType    string   `json:"content_type"`
	FinalName      string   `json:"final_name"`
	HasTranslation bool     `json:"has_translation"`
	Holiday        bool     `json:"holiday"`
	Language       string   `json:"language"`
	Lecturer       string   `json:"lecturer"`
	LessonID       string   `json:"lid"`
	ManualName     string   `json:"manual_name"`
	Number         int      `json:"number"`
	Part           int      `json:"part"`
	Pattern        string   `json:"pattern"`
	RequireTest    bool     `json:"require_test"`
	Sources        []string `json:"sources"`
	Tags           []string `json:"tags"`
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

func GetState() *CaptureState {
	cs := &CaptureState{}
	err := json.Unmarshal(Data, &cs)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("GetState:", cs)
	return cs
}

func SetState(c mqtt.Client, m mqtt.Message) {
	cs := &CaptureState{}
	err := json.Unmarshal(m.Payload(), &cs)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("SetState:", cs)
	Data = m.Payload()
}

func (m *MdbPayload) PostMDB(ep string) error {
	u, _ := json.Marshal(m)
	body := strings.NewReader(string(u))
	fmt.Println("MDB Payload:", body)
	req, err := http.NewRequest("POST", common.MdbUrl+ep, body)
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
		fmt.Println("Non-OK HTTP status:", res.StatusCode)
		return err
	}

	return nil
}

func GetCaptureState(src string) (*CaptureState, error) {
	req, err := http.NewRequest("GET", common.SdbUrl+src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	state := CaptureState{}
	err = json.Unmarshal(body, &state)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (w *WfdbCapture) GetWFDB(id string) error {
	req, err := http.NewRequest("GET", common.WfdbUrl+"/"+id, nil)
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

	err = json.Unmarshal(body, &w)
	if err != nil {
		return err
	}

	return nil
}

func (w *WfdbCapture) PutWFDB() error {
	u, _ := json.Marshal(w)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", common.WfdbUrl+w.CaptureID, body)
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
		fmt.Println("Non-OK HTTP status:", res.StatusCode)
		return err
	}

	return nil
}

func (w *CaptureFlow) PutFlow() error {
	u, _ := json.Marshal(w)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", common.WfApiUrl, body)
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
		fmt.Println("Non-OK HTTP status:", res.StatusCode)
		return err
	}

	return nil
}

func GetStationID(id string) string {
	switch id {
	case "mltmain":
		return common.MltMain
	case "mltbackup":
		return common.MltBackup
	case "maincap":
		return common.MainCap
	case "backupcap":
		return common.BackupCap
	}

	return ""
}

func GetDateFromID(id string) string {
	ts := strings.Split(id, "c")[1]
	tsInt, _ := strconv.ParseInt(ts, 10, 64)
	tsTime := time.Unix(0, tsInt*int64(time.Millisecond))
	return strings.Split(tsTime.String(), " ")[0]
}
