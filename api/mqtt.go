package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"github.com/Bnei-Baruch/exec-api/pkg/wf"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"net"
	"strings"
	"time"
)

type Mqtt struct {
	mqtt *paho.Client
	WF   wf.WF
}

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

type MQ interface {
	SendMessage(string, *MqttPayload)
	Init() error
	SendRespond(string, *MqttPayload)
}

func NewMqtt(mqtt *paho.Client) MQ {
	return &Mqtt{
		mqtt: mqtt,
	}
}

func (m *Mqtt) Init() error {
	m.mqtt = paho.NewClient(paho.ClientConfig{
		ClientID:      common.EP + "-exec_mqtt_client",
		OnClientError: m.lostMQTT,
	})

	m.WF = wf.NewWorkFlow(m.mqtt)

	if err := m.conMQTT(); err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("initialize mqtt connection")
	}

	return nil
}

func (m *Mqtt) conMQTT() error {
	var err error

	m.mqtt.Conn = connect()
	var sessionExpiryInterval = uint32(math.MaxUint32)

	cp := &paho.Connect{
		ClientID:     common.EP + "-exec_mqtt_client",
		KeepAlive:    10,
		CleanStart:   true,
		Username:     common.USERNAME,
		Password:     []byte(common.PASSWORD),
		UsernameFlag: true,
		PasswordFlag: true,
		Properties: &paho.ConnectProperties{
			SessionExpiryInterval: &sessionExpiryInterval,
		},
	}

	m.mqtt.SetErrorLogger(NewPahoLogAdapter(zerolog.DebugLevel))
	debugLog := NewPahoLogAdapter(zerolog.DebugLevel)
	m.mqtt.SetDebugLogger(debugLog)
	m.mqtt.PingHandler.SetDebug(debugLog)
	m.mqtt.Router.SetDebugLogger(debugLog)

	ca, err := m.mqtt.Connect(context.Background(), cp)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("client.Connect")
	}
	if ca.ReasonCode != 0 {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT connect error: %d - %s", ca.ReasonCode, ca.Properties.ReasonString)
	}

	sa, err := m.mqtt.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			common.ServiceTopic:  {QoS: byte(1)},
			common.WorkflowTopic: {QoS: byte(1)},
			common.StateTopic:    {QoS: byte(1)},
		},
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("client.Subscribe")
	}
	if sa.Reasons[0] != byte(1) {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT subscribe error: %d ", sa.Reasons[0])
	}

	m.mqtt.Router.RegisterHandler(common.ServiceTopic, m.execMessage)
	m.mqtt.Router.RegisterHandler(common.WorkflowTopic, m.WF.MqttMessage)
	m.mqtt.Router.RegisterHandler(common.StateTopic, m.WF.SetState)

	return nil
}

func connect() net.Conn {
	var conn net.Conn
	var err error

	for {
		conn, err = tls.Dial("tcp", common.SERVER, nil)
		if err != nil {
			log.Error().Str("source", "MQTT").Err(err).Msg("conn.Dial")
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}

	return conn
}

func (m *Mqtt) lostMQTT(err error) {
	log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
	time.Sleep(1 * time.Second)
	if err := m.mqtt.Disconnect(&paho.Disconnect{ReasonCode: 0}); err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Reconnecting..")
	}
	time.Sleep(1 * time.Second)
	_ = m.conMQTT()
}

func (m *Mqtt) execMessage(p *paho.Publish) {
	//var User = m.Properties.User
	//var CorrelationData = m.Properties.CorrelationData
	//var ResponseTopic = m.Properties.ResponseTopic

	log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", string(p.Payload), p.Topic)
	id := "false"
	s := strings.Split(p.Topic, "/")
	pl := string(p.Payload)

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	if id == "false" {
		switch pl {
		case "start":
			go m.startExecMqtt(pl)
		case "stop":
			go m.stopExecMqtt(pl)
		case "status":
			go m.execStatusMqtt(pl)
		}
	}

	if id != "false" {
		switch pl {
		case "start":
			go m.startExecMqttByID(pl, id)
		case "stop":
			go m.stopExecMqttByID(pl, id)
		case "status":
			go m.execStatusMqttByID(pl, id)
		case "cmdstat":
			go m.cmdStatMqtt(pl, id)
		case "progress":
			go m.getProgressMqtt(pl, id)
		case "report":
			go m.getReportMqtt(pl, id)
		case "alive":
			go m.isAliveMqtt(pl, id)
		}
	}
}

func (m *Mqtt) SendMessage(topic string, p *MqttPayload) {
	message, err := json.Marshal(p)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	}
	pa, err := m.mqtt.Publish(context.Background(), &paho.Publish{
		QoS:     byte(1),
		Retain:  false,
		Topic:   topic,
		Payload: message,
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Publish: Topic - " + topic + " " + pa.Properties.ReasonString)
	}

	log.Debug().Str("source", "MQTT").Str("json", string(message)).Msg("Publish: Topic - " + topic)
}

func (m *Mqtt) SendRespond(id string, p *MqttPayload) {
	var topic string

	if id == "false" {
		topic = common.ServiceDataTopic + common.EP
	} else {
		topic = common.ServiceDataTopic + common.EP + "/" + id
	}
	message, err := json.Marshal(p)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	}

	pa, err := m.mqtt.Publish(context.Background(), &paho.Publish{
		QoS:     byte(1),
		Retain:  false,
		Topic:   topic,
		Payload: message,
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT Publish error: %d ", pa.Properties.ReasonString)
	}
}

type PahoLogAdapter struct {
	level zerolog.Level
}

func NewPahoLogAdapter(level zerolog.Level) *PahoLogAdapter {
	return &PahoLogAdapter{level: level}
}

func (a *PahoLogAdapter) Println(v ...interface{}) {
	log.Debug().Str("source", "MQTT").Msgf("%s", fmt.Sprint(v...))
}

func (a *PahoLogAdapter) Printf(format string, v ...interface{}) {
	log.Debug().Str("source", "MQTT").Msgf("%s", fmt.Sprintf(format, v...))
}
