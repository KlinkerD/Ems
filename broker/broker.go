// Package broker definiert Nachrichten und die Schnittstelle zum
// Message-Broker. Implementierungen liegen in Unterpaketen wie broker/mqtt.
//
// Das Paket kennt keine fachlichen Topics. Es arbeitet mit contracts.Topic;
// welche Topics es gibt, steht im Schnittstellenvertrag (z. B. contracts/meter).
package broker

import (
	"encoding/json"

	"github.com/KlinkerD/Ems/contracts"
)

// Message ist eine Nachricht auf einem Topic mit JSON-Payload.
type Message struct {
	Topic   contracts.Topic
	Payload json.RawMessage
}

// MessageHandler verbindet sich mit dem Broker, veröffentlicht Nachrichten
// und abonniert Topics.
type MessageHandler interface {
	Publish(Message) error
	Unsubscribe(topic contracts.Topic) error
	Subscribe(topic contracts.Topic, handler func(Message)) error
	Connect() error
	Close()
}
