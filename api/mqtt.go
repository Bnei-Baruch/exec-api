package api

import (
	"encoding/json"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"os"
	"strings"
)

type MqttPayload struct {
	Action  string                 `json:"action"`
	Error   error                  `json:"error"`
	Message string                 `json:"message"`
	Result  string                 `json:"result"`
	Data    map[string]interface{} `json:"data"`
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

	if token := a.Msg.Subscribe("workflow/service/"+ep+"/#", byte(1), a.workflowMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: workflow/service/"+ep+"/#")
	}

	if token := a.Msg.Subscribe("kli/workflow/service/"+ep+"/#", byte(1), a.workflowMessage); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: kli/workflow/service/"+ep+"/#")
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

func (a *App) workflowMessage(c mqtt.Client, m mqtt.Message) {
	//fmt.Printf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	id := "false"
	s := strings.Split(m.Topic(), "/")
	ep := os.Getenv("MQTT_EP")

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "workflow" && len(s) == 4 {
		id = s[3]
	}

	mp := MqttPayload{}
	err := json.Unmarshal(m.Payload(), &mp)
	if err != nil {
		fmt.Println(err.Error())
	}

	if id != "false" {
		switch mp.Action {
		case "start":
			go a.startFlow(mp, id)
		case "stop":
			go a.stopFlow(ep, id)
		case "wflow":
			go a.wflowFlow(ep, id)
		}
	}
}

func (a *App) Publish(topic string, message string) {
	text := fmt.Sprintf(message)
	//a.Msg.Publish(topic, byte(1), false, text)
	if token := a.Msg.Publish(topic, byte(1), false, text); token.Wait() && token.Error() != nil {
		fmt.Printf("Publish message error: %s\n", token.Error())
	}
}
