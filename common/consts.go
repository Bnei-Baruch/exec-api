package common

import "os"

const RespondTopic = "exec/service/data/"

var (
	MltMain   = os.Getenv("MLT_MAIN")
	MltBackup = os.Getenv("MLT_BACKUP")
	MainCap   = os.Getenv("MAIN_CAP")
	BackupCap = os.Getenv("BACKUP_CAP")

	SdbUrl   = os.Getenv("SDB_URL")
	WfApiUrl = os.Getenv("WFAPI_URL")
	MdbUrl   = os.Getenv("MDB_URL")
	WfdbUrl  = os.Getenv("WFDB_URL")

	EP       = os.Getenv("MQTT_EP")
	SERVER   = os.Getenv("MQTT_URL")
	USERNAME = os.Getenv("MQTT_USER")
	PASSWORD = os.Getenv("MQTT_PASS")
)
