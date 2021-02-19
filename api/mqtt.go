package api

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"strings"
)

func (a *App) MsgHandler(c mqtt.Client, m mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	var id, ep string
	s := strings.Split(m.Topic(), "/")
	p := string(m.Payload())
	ep = s[2]
	if len(s) > 3 {
		id = s[3]
	}

	if p == "start" && id == "" {
		a.startExecMqtt(ep)
	} else if p == "start" && id != "" {
		a.startExecMqttByID(ep, id)
	} else if p == "stop" && id == "" {
		a.stopExecMqtt(ep)
	} else if p == "stop" && id != "" {
		a.stopExecMqttByID(ep, id)
	} else if p == "status" && id == "" {
		a.execStatusMqtt(ep)
	} else if p == "status" && id != "" {
		a.execStatusMqttByID(ep, id)
	} else if p == "cmdstat" && id != "" {
		a.cmdStatMqtt(ep, id)
	} else if p == "progress" && id != "" {
		a.getProgressMqtt(ep, id)
	} else if p == "report" && id != "" {
		a.getReportMqtt(ep, id)
	} else if p == "alive" && id != "" {
		a.isAliveMqtt(ep, id)
	}
}

func (a *App) Publish(topic string, message string) {
	text := fmt.Sprintf(message)
	token := a.Msg.Publish(topic, 1, false, text)
	token.Wait()
}
