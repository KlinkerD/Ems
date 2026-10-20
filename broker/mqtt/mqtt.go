package mqtt

import (
	"fmt"
	"modbus/broker"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const timeout = 5 * time.Second

type Handler struct {
	client mqtt.Client
	mu     sync.Mutex
	subs   map[broker.Topic]mqtt.MessageHandler
}

func New(opts *mqtt.ClientOptions) *Handler {
	h := &Handler{subs: map[broker.Topic]mqtt.MessageHandler{}}
	opts.SetAutoReconnect(true)
	// Jeder Callback in eigener Goroutine, damit Handler blockieren dürfen
	// (z. B. Subscribe im Callback oder Manager.Set).
	opts.SetOrderMatters(false)
	opts.SetOnConnectHandler(h.resubscribe)
	h.client = mqtt.NewClient(opts)
	return h
}

func (h *Handler) Connect() error {
	return wait(h.client.Connect())
}

func (h *Handler) Close() {
	h.client.Disconnect(0)
}

func (h *Handler) Publish(m broker.Message) error {
	return wait(h.client.Publish(string(m.Topic), 0, false, []byte(m.Payload)))
}

func wait(t mqtt.Token) error {
	if !t.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt: timeout after %s", timeout)
	}
	return t.Error()
}

func (h *Handler) Subscribe(topic broker.Topic, handler func(broker.Message)) error {
	cb := func(_ mqtt.Client, msg mqtt.Message) {
		handler(broker.Message{Topic: broker.Topic(msg.Topic()), Payload: msg.Payload()})
	}
	h.mu.Lock()
	h.subs[topic] = cb
	h.mu.Unlock()
	return wait(h.client.Subscribe(string(topic), 1, cb))
}

func (h *Handler) resubscribe(c mqtt.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for t, cb := range h.subs {
		c.Subscribe(string(t), 1, cb)
	}
}
