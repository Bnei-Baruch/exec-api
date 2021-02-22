package api

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"os"
	"strings"
)

func (a *App) SubMQTT(c mqtt.Client) {
	fmt.Println("- Connected to MQTT -")
	ep := os.Getenv("MQTT_EP")
	if token := a.Msg.Subscribe("exec/service/"+ep+"/#", byte(1), a.MsgHandler); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: exec/service/"+ep+"/#")
	}

	if token := a.Msg.Subscribe("kli/exec/service/"+ep+"/#", byte(1), a.MsgHandler); token.Wait() && token.Error() != nil {
		fmt.Printf("MQTT Subscription error: %s\n", token.Error())
	} else {
		fmt.Printf("%s\n", "MQTT Subscription: kli/exec/service/"+ep+"/#")
	}
}

func (a *App) LostMQTT(c mqtt.Client, e error) {
	fmt.Printf("MQTT Connection Error: %s\n", e)
}

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
