package api

import (
	"encoding/json"
	"fmt"
	wf "github.com/Bnei-Baruch/exec-api/pkg/workflow"
	"github.com/eclipse/paho.mqtt.golang"
	"os"
	"strings"
)

const RespondTopic = "exec/service/data/"

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
	ep := os.Getenv("MQTT_EP")
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

	if token := a.Msg.Subscribe("workflow/service/"+ep+"/#", byte(1), wf.MqttMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: workflow/service/"+ep+"/#")
	}

	if token := a.Msg.Subscribe("kli/workflow/service/"+ep+"/#", byte(1), wf.MqttMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: kli/workflow/service/"+ep+"/#")
	}

	if token := a.Msg.Subscribe("workflow/state/capture/"+ep+"/#", byte(1), wf.SetState); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: workflow/state/capture/"+ep+"/#")
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
	ep := os.Getenv("MQTT_EP")

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	if id == "false" {
		switch p {
		case "start":
			go a.startExecMqtt(ep)
		case "stop":
			go a.stopExecMqtt(ep)
		case "status":
			go a.execStatusMqtt(ep)
		}
	}

	if id != "false" {
		switch p {
		case "start":
			go a.startExecMqttByID(ep, id)
		case "stop":
			go a.stopExecMqttByID(ep, id)
		case "status":
			go a.execStatusMqttByID(ep, id)
		case "cmdstat":
			go a.cmdStatMqtt(ep, id)
		case "progress":
			go a.getProgressMqtt(ep, id)
		case "report":
			go a.getReportMqtt(ep, id)
		case "alive":
			go a.isAliveMqtt(ep, id)
		}
	}
}

func (a *App) SendRespond(ep string, id string, m *MqttPayload) {
	var topic string

	if id == "false" {
		topic = RespondTopic + ep
	} else {
		topic = RespondTopic + ep + "/" + id
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
