package workflow

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	MltMain   = os.Getenv("MLT_MAIN")
	MltBackup = os.Getenv("MLT_BACKUP")
	MainCap   = os.Getenv("MAIN_CAP")
	BackupCap = os.Getenv("BACKUP_CAP")

	SdbUrl   = os.Getenv("SDB_URL")
	WfApiUrl = os.Getenv("WFAPI_URL")
	MdbUrl   = os.Getenv("MDB_URL")
	WfdbUrl  = os.Getenv("WFDB_URL")
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

var Data = 10

func WorkflowState(c mqtt.Client, m mqtt.Message) {
	cs := &CaptureState{}
	err := json.Unmarshal(m.Payload(), &cs)
	fmt.Println("onMsg State:", cs)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func (m *MdbPayload) PostMDB(ep string) error {
	u, _ := json.Marshal(m)
	body := strings.NewReader(string(u))
	fmt.Println("MDB Payload:", body)
	r, err := http.NewRequest("POST", MdbUrl+ep, body)
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(r)
	defer response.Body.Close()
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", response.StatusCode)
		return err
	}

	return nil
}

func GetCaptureState(src string) (*CaptureState, error) {
	req, err := http.NewRequest("GET", SdbUrl+src, nil)
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
	req, err := http.NewRequest("GET", WfdbUrl+"/"+id, nil)
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
	req, err := http.NewRequest("PUT", WfdbUrl+w.CaptureID, body)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(req)
	defer response.Body.Close()
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", response.StatusCode)
		return err
	}

	return nil
}

func (w *CaptureFlow) PutFlow() error {
	u, _ := json.Marshal(w)
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", WfApiUrl, body)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(req)
	defer response.Body.Close()
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", response.StatusCode)
		return err
	}

	return nil
}

func GetStationID(id string) string {
	switch id {
	case "mltmain":
		return MltMain
	case "mltbackup":
		return MltBackup
	case "maincap":
		return MainCap
	case "backupcap":
		return BackupCap
	}

	return ""
}

func GetDateFromID(id string) string {
	ts := strings.Split(id, "c")[1]
	tsInt, _ := strconv.ParseInt(ts, 10, 64)
	tsTime := time.Unix(0, tsInt*int64(time.Millisecond))
	return strings.Split(tsTime.String(), " ")[0]
}

func delReq(ep string) error {
	server := os.Getenv("MDB_URL")
	req, err := http.NewRequest("DELETE", server+"/"+ep, nil)
	client := &http.Client{}
	response, err := client.Do(req)
	defer response.Body.Close()
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", response.StatusCode)
	}

	return nil
}

func writeToLog(path string, line string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening file: ", err)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Println(line)

	return nil
}
