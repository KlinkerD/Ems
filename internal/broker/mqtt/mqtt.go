// Package mqtt implementiert broker.MessageHandler mit dem Paho-MQTT-Client.
package mqtt

import (
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"modbus/internal/broker"
)

const timeout = 5 * time.Second

// Handler ist ein broker.MessageHandler für MQTT. Abos werden gemerkt und
// nach einem Reconnect automatisch erneuert.
type Handler struct {
	client paho.Client
	mu     sync.Mutex
	subs   map[broker.Topic]paho.MessageHandler
}

var _ broker.MessageHandler = (*Handler)(nil)

// New erzeugt einen Handler. Auto-Reconnect und der OnConnect-Handler in opts
// werden dabei überschrieben.
func New(opts *paho.ClientOptions) *Handler {
	h := &Handler{subs: map[broker.Topic]paho.MessageHandler{}}
	opts.SetAutoReconnect(true)
	// Jeder Callback in eigener Goroutine, damit Handler blockieren dürfen
	// (z. B. Subscribe im Callback oder Manager.Set).
	opts.SetOrderMatters(false)
	opts.SetOnConnectHandler(h.resubscribe)
	h.client = paho.NewClient(opts)
	return h
}

// Connect verbindet mit dem Broker.
func (h *Handler) Connect() error {
	return wait(h.client.Connect())
}

// Close trennt die Verbindung sofort.
func (h *Handler) Close() {
	h.client.Disconnect(0)
}

// Publish veröffentlicht m mit QoS 0.
func (h *Handler) Publish(m broker.Message) error {
	return wait(h.client.Publish(string(m.Topic), 0, false, []byte(m.Payload)))
}

func wait(t paho.Token) error {
	if !t.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt: timeout nach %s", timeout)
	}
	return t.Error()
}

// Subscribe abonniert topic mit QoS 1. Ein erneuter Aufruf für dasselbe Topic
// ersetzt den Handler.
func (h *Handler) Subscribe(topic broker.Topic, handler func(broker.Message)) error {
	cb := func(_ paho.Client, msg paho.Message) {
		handler(broker.Message{Topic: broker.Topic(msg.Topic()), Payload: msg.Payload()})
	}
	h.mu.Lock()
	h.subs[topic] = cb
	h.mu.Unlock()
	return wait(h.client.Subscribe(string(topic), 1, cb))
}

func (h *Handler) resubscribe(c paho.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for t, cb := range h.subs {
		c.Subscribe(string(t), 1, cb)
	}
}

func (h *Handler) Unsubscribe(topic broker.Topic) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.subs[topic]
	if !ok {
		return nil
	}
	delete(h.subs, topic)
	return wait(h.client.Unsubscribe(string(topic)))
}
