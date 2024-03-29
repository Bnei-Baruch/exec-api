package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/wf"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"strings"
)

var MQTT mqtt.Client

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

type PahoLogAdapter struct {
	level log.Level
}

func NewPahoLogAdapter(level log.Level) *PahoLogAdapter {
	return &PahoLogAdapter{level: level}
}

func (a *PahoLogAdapter) Println(v ...interface{}) {
	log.Infof("MQTT: %s", fmt.Sprint(v...))
}

func (a *PahoLogAdapter) Printf(format string, v ...interface{}) {
	log.Infof("MQTT: %s", fmt.Sprintf(format, v...))
}

func InitMQTT() error {
	log.Info("[InitMQTT] Init")
	//mqtt.DEBUG = NewPahoLogAdapter(log.DebugLevel)
	//mqtt.WARN = NewPahoLogAdapter(log.WarnLevel)
	mqtt.CRITICAL = NewPahoLogAdapter(log.PanicLevel)
	mqtt.ERROR = NewPahoLogAdapter(log.ErrorLevel)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(viper.GetString("mqtt.url"))
	opts.SetClientID(viper.GetString("mqtt.client_id"))
	opts.SetUsername(viper.GetString("mqtt.user"))
	opts.SetPassword(viper.GetString("mqtt.password"))
	opts.SetAutoReconnect(true)
	opts.SetOnConnectHandler(SubMQTT)
	opts.SetConnectionLostHandler(LostMQTT)
	opts.SetBinaryWill(viper.GetString("mqtt.status_topic"), []byte("Offline"), byte(1), true)
	MQTT = mqtt.NewClient(opts)
	if token := MQTT.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func SubMQTT(c mqtt.Client) {
	if token := MQTT.Publish(viper.GetString("mqtt.status_topic"), byte(1), true, []byte("Online")); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] notify status error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] notify status to: %s", viper.GetString("mqtt.status_topic"))
	}

	ExecServiceTopic := viper.GetString("mqtt.exec_service_topic")
	if token := MQTT.Subscribe(ExecServiceTopic, byte(1), ExecMessage); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] Subscribe error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] Subscribed to: %s", ExecServiceTopic)
	}

	ExecStateTopic := viper.GetString("mqtt.exec_state_topic")
	if token := MQTT.Subscribe(ExecStateTopic, byte(1), ExecState); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] Subscribe error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] Subscribed to: %s", ExecStateTopic)
	}

	WorkflowServiceTopic := viper.GetString("mqtt.wf_service_topic")
	if token := MQTT.Subscribe(WorkflowServiceTopic, byte(1), wf.MqttMessage); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] Subscribe error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] Subscribed to: %s", WorkflowServiceTopic)
	}

	WorkflowStateTopic := viper.GetString("mqtt.wf_state_topic")
	if token := MQTT.Subscribe(WorkflowStateTopic, byte(1), wf.SetState); token.Wait() && token.Error() != nil {
		log.Errorf("[SubMQTT] Subscribe error: %s", token.Error())
	} else {
		log.Infof("[SubMQTT] Subscribed to: %s", WorkflowStateTopic)
	}
}

func LostMQTT(c mqtt.Client, err error) {
	log.Errorf("[LostMQTT] Lost connection: %s", err)
}

func ExecMessage(c mqtt.Client, m mqtt.Message) {
	log.Debugf("[ExecMessage] topic: %s |  message: %s", m.Topic(), string(m.Payload()))

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
			go startExecMqtt(p)
		case "stop":
			go stopExecMqtt(p)
		case "status":
			go execStatusMqtt(p)
		}
	}

	if id != "false" {
		switch p {
		case "start":
			go startExecMqttByID(p, id)
		case "stop":
			go stopExecMqttByID(p, id)
		case "status":
			go execStatusMqttByID(p, id)
		case "cmdstat":
			go cmdStatMqtt(p, id)
		case "progress":
			go getProgressMqtt(p, id)
		case "report":
			go getReportMqtt(p, id)
		case "alive":
			go isAliveMqtt(p, id)
		}
	}
}

func SendRespond(id string, m *MqttPayload) {
	var topic string
	ExecDataTopic := viper.GetString("mqtt.exec_data_topic")
	ClientID := viper.GetString("mqtt.client_id")

	if id == "false" {
		topic = ExecDataTopic + ClientID
	} else {
		topic = ExecDataTopic + ClientID + "/" + id
	}
	message, err := json.Marshal(m)
	if err != nil {
		log.Errorf("Message parsing: %s", err)
	}

	text := fmt.Sprintf(string(message))
	log.Debugf("[SendRespond] topic: %s |  message: %s", topic, string(message))
	if token := MQTT.Publish(topic, byte(1), false, text); token.Wait() && token.Error() != nil {
		log.Errorf("Send Respond: %s", err)
	}
}

func SendState(topic string, state string) {
	log.Debugf("[SendState] topic: %s |  message: %s", topic, state)

	if token := MQTT.Publish(topic, byte(1), true, state); token.Wait() && token.Error() != nil {
		log.Errorf("Send State: %s", token.Error())
	}
}

func SendMessage(topic string, p *MqttPayload) {
	message, err := json.Marshal(p)
	if err != nil {
		log.Errorf("Message parsing: %s", err)
	}

	log.Debugf("[SendMessage] topic: %s |  message: %s", topic, message)

	if token := MQTT.Publish(topic, byte(1), false, message); token.Wait() && token.Error() != nil {
		log.Errorf("Send State: %s", token.Error())
	}
}
