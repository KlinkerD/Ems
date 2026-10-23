// Package broker definiert Nachrichten und die Schnittstelle zum
// Message-Broker. Implementierungen liegen in Unterpaketen wie broker/mqtt.
package broker

import (
	"encoding/json"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// Topic ist das Topic aus dem Schnittstellenvertrag. Die konkreten Topics
// stehen in contracts/meter.
type Topic = contract.Topic

// Message ist eine Nachricht auf einem Topic mit JSON-Payload.
type Message struct {
	Topic   Topic
	Payload json.RawMessage
}

// MessageHandler verbindet sich mit dem Broker, veröffentlicht Nachrichten
// und abonniert Topics.
type MessageHandler interface {
	Publish(Message) error
	Unsubscribe(topic Topic) error
	Subscribe(topic Topic, handler func(Message)) error
	Connect() error
	Close()
}
