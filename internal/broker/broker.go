// Package broker definiert die Topics, Nachrichten und die Schnittstelle zum
// Message-Broker. Implementierungen liegen in Unterpaketen wie broker/mqtt.
package broker

import (
	"encoding/json"
	"strings"
)

// Topic ist ein Broker-Topic. Der Platzhalter {id} wird mit ForMeter ersetzt.
type Topic string

const (
	TopicMeterValues Topic = "meter/{id}/values"
	TopicMeterStatus Topic = "meter/{id}/status"
	TopicMeterError  Topic = "meter/{id}/error"
	TopicMeterConfig Topic = "meter/{id}/config"
	TopicMeterInfo   Topic = "meter/{id}/info"
	TopicConfig      Topic = "config"
)

// ForMeter setzt die Meter-ID in den Platzhalter {id} ein.
func (t Topic) ForMeter(id string) Topic {
	return Topic(strings.Replace(string(t), "{id}", id, 1))
}

// Message ist eine Nachricht auf einem Topic mit JSON-Payload.
type Message struct {
	Topic   Topic
	Payload json.RawMessage
}

// MeterIDs ist der Payload auf TopicConfig: die IDs aller Meter, deren
// Einzel-Config auf TopicMeterConfig abonniert werden soll.
type MeterIDs struct {
	Version int      `json:"version"`
	Meters  []string `json:"meters"`
}

// MessageHandler verbindet sich mit dem Broker, veröffentlicht Nachrichten
// und abonniert Topics.
type MessageHandler interface {
	Publish(Message) error
	Subscribe(topic Topic, handler func(Message)) error
	Connect() error
	Close()
}
