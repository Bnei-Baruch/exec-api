package api

import (
	"encoding/json"
	"github.com/Bnei-Baruch/exec-api/pkg/workflow"
)

func (a *App) startFlow(mp MqttPayload, id string) {

	wp := workflow.WorkflowPayload{}
	rp := MqttPayload{}

	u, _ := json.Marshal(wp)

	err := workflow.MdbWrite(rp.Action, string(u))
	if err != nil {
		rp.Error = err
		rp.Message = "MDB Request Failed"
		m, _ := json.Marshal(rp)
		a.Publish("workflow/service/data/"+rp.Action, string(m))
		return
	}

	err = workflow.WfdbWrite(rp.Action, string(u))
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

func (a *App) stopFlow(ep string, id string) {
	return
}

func (a *App) wflowFlow(ep string, id string) {
	return
}
