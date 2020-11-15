package cmd

import (
	"os"

	"github.com/Bnei-Baruch/exec-api/api"
)

func Init() {
	listenAddress := os.Getenv("LISTEN_ADDRESS")
	accountsUrl := os.Getenv("ACC_URL")
	skipAuth := os.Getenv("SKIP_AUTH") == "true"

	a := api.App{}
	a.Initialize(accountsUrl, skipAuth)
	a.Run(listenAddress)
}
