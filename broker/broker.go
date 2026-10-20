package broker

import (
	"encoding/json"
	"strings"
)

type Topic string

const (
	METER_VALUES Topic = "meter/{id}/values"
	METER_STATUS Topic = "meter/{id}/status"
	METER_ERROR  Topic = "meter/{id}/error"
	METER_CONFIG Topic = "meter/{id}/config"
	METER_INFO   Topic = "meter/{id}/info"
	CONFIG       Topic = "config"
)

func (t Topic) ForMeter(id string) Topic {
	return Topic(strings.Replace(string(t), "{id}", id, 1))
}

type Message struct {
	Topic   Topic
	Payload json.RawMessage
}

type MessageHandler interface {
	Publish(Message) error
	Subscribe(topic Topic, handler func(Message)) error
	Connect() error
	Close()
}
