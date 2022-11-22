package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/exec-api/common"
	"github.com/Bnei-Baruch/exec-api/pkg/wf"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/url"
	"strings"
	"time"
)

type Mqtt struct {
	mqtt *autopaho.ConnectionManager
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

func NewMqtt(mqtt *autopaho.ConnectionManager) MQ {
	return &Mqtt{
		mqtt: mqtt,
	}
}

func (m *Mqtt) Init() error {
	log.Info().Str("source", "APP").Msgf("Init MQTT")

	serverURL, err := url.Parse(common.SERVER)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("environmental variable must be a valid URL")
	}

	cliCfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{serverURL},
		KeepAlive:         10,
		ConnectRetryDelay: 3 * time.Second,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			log.Info().Str("source", "APP").Msgf("MQTT connection up")
			if _, err := cm.Subscribe(context.Background(), &paho.Subscribe{
				Subscriptions: map[string]paho.SubscribeOptions{
					common.ExecServiceTopic:     {QoS: byte(1)},
					common.ExecStateTopic:       {QoS: byte(1)},
					common.WorkflowServiceTopic: {QoS: byte(1)},
					common.WorkflowStateTopic:   {QoS: byte(1)},
				},
			}); err != nil {
				log.Error().Str("source", "MQTT").Err(err).Msg("client.Subscribe")
				return
			}
			log.Info().Str("source", "APP").Msgf("MQTT subscription made")
		},
		OnConnectError: func(err error) { log.Error().Str("source", "MQTT").Err(err).Msg("error whilst attempting connection") },
		ClientConfig: paho.ClientConfig{
			ClientID: common.EP + "-exec_mqtt_client",
			//Router: paho.RegisterHandler(common.WorkflowExec, m.execMessage),
			Router:        paho.NewStandardRouter(),
			OnClientError: func(err error) { log.Error().Str("source", "MQTT").Err(err).Msg("server requested disconnect:") },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT server requested disconnect: %d", d.Properties.ReasonString)
				} else {
					log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT server requested disconnect: %d", d.ReasonCode)
				}
			},
		},
	}

	cliCfg.SetUsernamePassword(common.USERNAME, []byte(common.PASSWORD))

	debugLog := NewPahoLogAdapter(zerolog.DebugLevel)

	cliCfg.Debug = debugLog
	cliCfg.PahoDebug = debugLog

	m.mqtt, err = autopaho.NewConnection(context.Background(), cliCfg)
	if err != nil {
		return err
	}

	m.WF = wf.NewWorkFlow(m.mqtt)

	cliCfg.Router.RegisterHandler(common.ExecServiceTopic, m.execMessage)
	cliCfg.Router.RegisterHandler(common.ExecStateTopic, m.execState)
	cliCfg.Router.RegisterHandler(common.WorkflowServiceTopic, m.WF.MqttMessage)
	cliCfg.Router.RegisterHandler(common.WorkflowStateTopic, m.WF.SetState)

	return nil
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

func (m *Mqtt) SendState(topic string, state string) {
	//message, err := json.Marshal(state)
	//if err != nil {
	//	log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	//}
	pa, err := m.mqtt.Publish(context.Background(), &paho.Publish{
		QoS:     byte(1),
		Retain:  true,
		Topic:   topic,
		Payload: []byte(state),
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Publish: Topic - " + topic + " " + pa.Properties.ReasonString)
	}

	log.Debug().Str("source", "MQTT").Str("State", state).Msg("Publish: Topic - " + topic)
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
