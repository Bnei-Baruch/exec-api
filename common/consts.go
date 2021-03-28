package common

import "os"

var (
	MltMain   = os.Getenv("MLT_MAIN")
	MltBackup = os.Getenv("MLT_BACKUP")
	MainCap   = os.Getenv("MAIN_CAP")
	BackupCap = os.Getenv("BACKUP_CAP")
	ArchCap   = os.Getenv("ARCH_CAP")

	SdbUrl   = os.Getenv("SDB_URL")
	WfApiUrl = os.Getenv("WFAPI_URL")
	MdbUrl   = os.Getenv("MDB_URL")
	WfdbUrl  = os.Getenv("WFDB_URL")

	EP       = os.Getenv("MQTT_EP")
	SERVER   = os.Getenv("MQTT_URL")
	USERNAME = os.Getenv("MQTT_USER")
	PASSWORD = os.Getenv("MQTT_PASS")

	WFCAP = os.Getenv("WF_CAP")

	ServiceTopic  = "exec/service/" + EP + "/#"
	WorkflowTopic = "workflow/service/capture/" + EP
	StateTopic    = "workflow/state/capture/" + WFCAP
)

const (
	ExtPrefix         = "kli/"
	ServiceDataTopic  = "exec/service/data/"
	WorkflowDataTopic = "workflow/service/data/"
	CapturedPath      = "/opt/backup/"
)
