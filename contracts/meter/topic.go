// Package meter beschreibt die MQTT-Schnittstelle rund um Zähler: welche
// Topics es gibt und welcher Payload darauf liegt.
//
// Regeln für Änderungen: Felder und Topics nur hinzufügen, nie entfernen oder
// umbenennen, solange ein Service die alte Version nutzt. Neue Felder sind
// optional (omitempty).
package meter

import "strings"

// Topic ist ein Broker-Topic. Der Platzhalter {id} wird mit ForMeter ersetzt.
type Topic string

const (
	// TopicMeterIDs trägt MeterIDs: welche Meter aktiv sein sollen.
	TopicMeterIDs Topic = "config"
	// TopicConfig trägt die Config eines einzelnen Meters.
	TopicConfig Topic = "meter/{id}/config"
	// TopicValues trägt []Value, einmal pro Poll-Intervall.
	TopicValues Topic = "meter/{id}/values"
	// TopicInfo trägt DeviceInfo, bei jedem Verbindungsaufbau.
	TopicInfo Topic = "meter/{id}/info"
	// TopicError trägt Error, wenn das Lesen fehlschlägt.
	TopicError Topic = "meter/{id}/error"
	// TopicStatus trägt Status, wenn sich der Zustand eines Meters ändert.
	TopicStatus Topic = "meter/{id}/status"
)

// ForMeter setzt die Meter-ID in den Platzhalter {id} ein.
func (t Topic) ForMeter(id string) Topic {
	return Topic(strings.Replace(string(t), "{id}", id, 1))
}
