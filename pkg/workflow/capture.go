package workflow

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type MdbPayload struct {
	CaptureSource string `json:"capture_source"`
	Station       string `json:"station"`
	User          string `json:"user"`
	FileName      string `json:"file_name"`
	WorkflowID    string `json:"workflow_id"`
}

type WorkflowPayload struct {
	CaptureID  string   `json:"capture_id"`
	Date       string   `json:"date"`
	StartName  string   `json:"start_name"`
	CaptureSrc string   `json:"capture_src"`
	Wfstatus   Wfstatus `json:"wfstatus"`
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

type Capture struct {
	ID        int                    `json:"id"`
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

func MdbWrite(ep string, payload string) error {
	server := os.Getenv("WFDB_URL")
	body := strings.NewReader(payload)
	req, err := http.NewRequest("PUT", server+"/"+ep, body)
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
