// Package broker definiert Nachrichten und die Schnittstelle zum
// Message-Broker. Implementierungen liegen in Unterpaketen wie broker/mqtt.
//
// Das Paket kennt keine fachlichen Topics. Welche Topics es gibt, steht im
// Schnittstellenvertrag (z. B. contracts/meter).
package broker

import "encoding/json"

// Topic ist ein Broker-Topic, z. B. "meter/umg-1/values".
type Topic string

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
