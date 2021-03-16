package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	wf "github.com/Bnei-Baruch/exec-api/pkg/workflow"
	"github.com/eclipse/paho.mqtt.golang"
	"strings"
)

type MqttPayload struct {
	Action  string      `json:"action,omitempty"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	Source  string      `json:"src,omitempty"`
	Error   error       `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Result  string      `json:"result,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (a *App) SubMQTT(c mqtt.Client) {
	fmt.Println("- Connected to MQTT -")
	ep := common.EP
	if token := a.Msg.Subscribe("exec/service/"+ep+"/#", byte(1), a.execMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: exec/service/"+ep+"/#")
	}

	if token := a.Msg.Subscribe("kli/exec/service/"+ep+"/#", byte(1), a.execMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: kli/exec/service/"+ep+"/#")
	}

	if token := a.Msg.Subscribe("workflow/service/capture/"+ep, byte(1), wf.MqttMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: workflow/service/capture/"+ep)
	}

	if token := a.Msg.Subscribe("kli/workflow/service/capture/"+ep, byte(1), wf.MqttMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: kli/workflow/service/capture/"+ep)
	}

	if token := a.Msg.Subscribe("workflow/state/capture/"+common.WFCAP, byte(1), wf.SetState); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: workflow/state/capture/#")
	}
}

func (a *App) LostMQTT(c mqtt.Client, e error) {
	fmt.Printf("MQTT Connection Error: %s\n", e)
}

func (a *App) execMessage(c mqtt.Client, m mqtt.Message) {
	//fmt.Printf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	id := "false"
	s := strings.Split(m.Topic(), "/")
	p := string(m.Payload())

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	if id == "false" {
		switch p {
		case "start":
			go a.startExecMqtt(p)
		case "stop":
			go a.stopExecMqtt(p)
		case "status":
			go a.execStatusMqtt(p)
		}
	}

	if id != "false" {
		switch p {
		case "start":
			go a.startExecMqttByID(p, id)
		case "stop":
			go a.stopExecMqttByID(p, id)
		case "status":
			go a.execStatusMqttByID(p, id)
		case "cmdstat":
			go a.cmdStatMqtt(p, id)
		case "progress":
			go a.getProgressMqtt(p, id)
		case "report":
			go a.getReportMqtt(p, id)
		case "alive":
			go a.isAliveMqtt(p, id)
		}
	}
}

func (a *App) SendRespond(id string, m *MqttPayload) {
	var topic string

	if id == "false" {
		topic = common.RespondTopic + common.EP
	} else {
		topic = common.RespondTopic + common.EP + "/" + id
	}
	message, err := json.Marshal(m)
	if err != nil {
		fmt.Printf("Files respond with json: %s\n", err)
	}

	text := fmt.Sprintf(string(message))
	if token := a.Msg.Publish(topic, byte(1), false, text); token.Wait() && token.Error() != nil {
		fmt.Printf("Send Respond error: %s\n", token.Error())
	}
}
