// Package meter beschreibt die MQTT-Schnittstelle rund um Zähler: welche
// Topics es gibt und welcher Payload darauf liegt.
//
// Regeln für Änderungen: Felder und Topics nur hinzufügen, nie entfernen oder
// umbenennen, solange ein Service die alte Version nutzt. Neue Felder sind
// optional (omitempty).
package meter

import (
	"strings"

	"github.com/KlinkerD/Ems/contracts"
)

// ModbusTopic ist eine Topic-Vorlage mit dem Platzhalter {id}. ForMeter macht
// daraus ein konkretes contracts.Topic für den Broker.
type ModbusTopic contracts.Topic

// TopicMeterIDs trägt MeterIDs: welche Meter aktiv sein sollen. Es hat keinen
// Platzhalter und ist deshalb direkt ein konkretes Topic.
const TopicMeterIDs contracts.Topic = "config"

const (
	// TopicConfig trägt die Config eines einzelnen Meters.
	TopicConfig ModbusTopic = "meter/{id}/config"
	// TopicValues trägt []Value, einmal pro Poll-Intervall.
	TopicValues ModbusTopic = "meter/{id}/values"
	// TopicInfo trägt DeviceInfo, bei jedem Verbindungsaufbau.
	TopicInfo ModbusTopic = "meter/{id}/info"
	// TopicError trägt Error, wenn das Lesen fehlschlägt.
	TopicError ModbusTopic = "meter/{id}/error"
	// TopicStatus trägt Status, wenn sich der Zustand eines Meters ändert.
	TopicStatus ModbusTopic = "meter/{id}/status"
)

// ForMeter setzt die Meter-ID in den Platzhalter {id} ein und liefert das
// konkrete Topic.
func (t ModbusTopic) ForMeter(id string) contracts.Topic {
	return contracts.Topic(strings.Replace(string(t), "{id}", id, 1))
}
