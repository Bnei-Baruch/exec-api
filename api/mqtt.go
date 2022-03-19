package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"net"
	"strings"
	"time"
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

func (a *App) ConMQTT() error {
	var err error

	a.Msg.Conn = connect()
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

	a.Msg.SetErrorLogger(NewPahoLogAdapter(zerolog.DebugLevel))
	debugLog := NewPahoLogAdapter(zerolog.DebugLevel)
	a.Msg.SetDebugLogger(debugLog)
	a.Msg.PingHandler.SetDebug(debugLog)
	a.Msg.Router.SetDebugLogger(debugLog)

	ca, err := a.Msg.Connect(context.Background(), cp)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("client.Connect")
	}
	if ca.ReasonCode != 0 {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT connect error: %d - %s", ca.ReasonCode, ca.Properties.ReasonString)
	}

	sa, err := a.Msg.Subscribe(context.Background(), &paho.Subscribe{
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

	a.Msg.Router.RegisterHandler(common.ServiceTopic, a.execMessage)
	a.Msg.Router.RegisterHandler(common.WorkflowTopic, a.MqttMessage)
	a.Msg.Router.RegisterHandler(common.StateTopic, SetState)

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

func (a *App) LostMQTT(err error) {
	log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
	time.Sleep(1 * time.Second)
	if err := a.Msg.Disconnect(&paho.Disconnect{ReasonCode: 0}); err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Reconnecting..")
	}
	time.Sleep(1 * time.Second)
	a.initMQTT()
}

func (a *App) execMessage(m *paho.Publish) {
	log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", string(m.Payload), m.Topic)
	id := "false"
	s := strings.Split(m.Topic, "/")
	p := string(m.Payload)

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

func (a *App) SendMessage(topic string, m *MqttWorkflow) {
	message, err := json.Marshal(m)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	}
	pa, err := a.Msg.Publish(context.Background(), &paho.Publish{
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

func (a *App) SendRespond(id string, m *MqttPayload) {
	var topic string

	if id == "false" {
		topic = common.ServiceDataTopic + common.EP
	} else {
		topic = common.ServiceDataTopic + common.EP + "/" + id
	}
	message, err := json.Marshal(m)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	}

	pa, err := a.Msg.Publish(context.Background(), &paho.Publish{
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
