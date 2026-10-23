// Package mqtt implementiert broker.MessageHandler mit dem Paho-MQTT-Client.
package mqtt

import (
	"errors"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/KlinkerD/Ems/broker"
	"github.com/KlinkerD/Ems/contracts"
)

const timeout = 5 * time.Second

// Handler ist ein broker.MessageHandler für MQTT. Abos werden gemerkt und
// nach einem Reconnect automatisch erneuert.
type Handler struct {
	client paho.Client
	mu     sync.Mutex
	subs   map[contracts.Topic]paho.MessageHandler
}

var _ broker.MessageHandler = (*Handler)(nil)

// Config enthält alles, was für die Verbindung zum MQTT-Broker nötig ist.
type Config struct {
	// BrokerURL ist die Adresse des Brokers, z. B. "tcp://localhost:1883".
	BrokerURL string
	// ClientID muss pro verbundenem Client eindeutig sein.
	ClientID string
	// Username und Password sind optional.
	Username string
	Password string
}

// New erzeugt einen Handler. Verbunden wird erst mit Connect.
func New(cfg Config) (*Handler, error) {
	if cfg.BrokerURL == "" {
		return nil, errors.New("mqtt: BrokerURL fehlt")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("mqtt: ClientID fehlt")
	}

	h := &Handler{subs: map[contracts.Topic]paho.MessageHandler{}}
	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		// Jeder Callback in eigener Goroutine, damit Handler blockieren dürfen
		// (z. B. Subscribe im Callback).
		SetOrderMatters(false).
		SetOnConnectHandler(h.resubscribe)
	h.client = paho.NewClient(opts)
	return h, nil
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
func (h *Handler) Subscribe(topic contracts.Topic, handler func(broker.Message)) error {
	cb := func(_ paho.Client, msg paho.Message) {
		handler(broker.Message{Topic: contracts.Topic(msg.Topic()), Payload: msg.Payload()})
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

func (h *Handler) Unsubscribe(topic contracts.Topic) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.subs[topic]
	if !ok {
		return nil
	}
	delete(h.subs, topic)
	return wait(h.client.Unsubscribe(string(topic)))
}
