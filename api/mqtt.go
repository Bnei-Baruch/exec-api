package api

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"os"
	"strings"
)

func (a *App) MsgHandler(c mqtt.Client, m mqtt.Message) {
	//fmt.Printf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	var id = "false"
	s := strings.Split(m.Topic(), "/")
	p := string(m.Payload())
	ep := os.Getenv("MQTT_EP")

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	if p == "start" && id == "false" {
		go a.startExecMqtt(ep)
	} else if p == "start" && id != "false" {
		go a.startExecMqttByID(ep, id)
	} else if p == "stop" && id == "false" {
		go a.stopExecMqtt(ep)
	} else if p == "stop" && id != "false" {
		go a.stopExecMqttByID(ep, id)
	} else if p == "status" && id == "false" {
		go a.execStatusMqtt(ep)
	} else if p == "status" && id != "false" {
		go a.execStatusMqttByID(ep, id)
	} else if p == "cmdstat" && id != "false" {
		go a.cmdStatMqtt(ep, id)
	} else if p == "progress" && id != "false" {
		go a.getProgressMqtt(ep, id)
	} else if p == "report" && id != "false" {
		go a.getReportMqtt(ep, id)
	} else if p == "alive" && id != "false" {
		go a.isAliveMqtt(ep, id)
	}
}

func (a *App) Publish(topic string, message string) {
	text := fmt.Sprintf(message)
	//a.Msg.Publish(topic, byte(1), false, text)
	if token := a.Msg.Publish(topic, byte(1), false, text); token.Wait() && token.Error() != nil {
		fmt.Printf("Publish message error: %s\n", token.Error())
	}
}
