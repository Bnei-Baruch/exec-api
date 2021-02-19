package api

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
	"time"
)

type MQTTListener struct {
	client mqtt.Client
}

func NewMQTTListener() *MQTTListener {
	return &MQTTListener{}
}

func (l *MQTTListener) init() error {
	server := os.Getenv("MQTT_URL")
	username := os.Getenv("MQTT_USER")
	password := os.Getenv("MQTT_PASS")

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("ssl://%s", server))
	opts.SetClientID("exec_mqtt_client")
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	l.client = mqtt.NewClient(opts)
	if token := l.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	ep := strings.Split(msg.Topic(), "/")[2]
	fmt.Printf("Connected %s\n", ep)
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
	sub(client)
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}

func sub(client mqtt.Client) {
	topic := "exec/service/#"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic: %s", topic)
}

func (a *App) initMQTT() {
	if os.Getenv("MQTT_URL") != "" {
		a.mqttListener = NewMQTTListener()
		if err := a.mqttListener.init(); err != nil {
			log.Fatal().Err(err).Msg("initialize mqtt listener")
		}
		time.Sleep(2 * time.Second)
		a.Publish(`{"message":"test"}`)
	}
}

func (a *App) Publish(message string) {
	text := fmt.Sprintf(message)
	token := a.mqttListener.client.Publish("exec/service/test", 1, false, text)
	token.Wait()
}
