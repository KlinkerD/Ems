package meter

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// MeterIDs ist der Payload auf TopicMeterIDs: die IDs aller Meter, deren
// Einzel-Config auf TopicConfig abonniert werden soll.
type MeterIDs struct {
	Version int       `json:"version"`
	Meters  []MeterID `json:"meters"`
}

// Value ist ein einzelner Messwert. TopicValues trägt eine Liste davon.
type Value struct {
	MeterID   MeterID   `json:"meterId"`
	Timestamp time.Time `json:"timestamp"`
	Type      Type      `json:"type"`
	Value     float64   `json:"value"`
}

// DeviceInfo ist der Payload auf TopicInfo. Er wird bei jedem
// Verbindungsaufbau zum Gerät gesendet.
type DeviceInfo struct {
	MeterID      MeterID `json:"meterId"`
	SerialNumber string  `json:"serialNumber,omitempty"`
	Manufacturer string  `json:"manufacturer,omitempty"`
	Model        string  `json:"model,omitempty"`
	Firmware     string  `json:"firmware,omitempty"`
}

// Error ist der Payload auf TopicError, z. B. wenn das Gerät nicht antwortet.
type Error struct {
	MeterID   MeterID   `json:"meterId"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Status ist der Payload auf TopicStatus, z. B. "Started" oder "Invalid Config".
type Status struct {
	MeterID   MeterID   `json:"meterId"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// MeterID identifiziert einen Zähler. Sie wird nur über NewMeterID oder
// MustMeterID erzeugt und ist dann nie leer. In JSON erscheint sie als
// einfacher String.
type MeterID struct {
	value string
}

var ErrInvalidMeterID = errors.New("ungültige meter-id")

// NewMeterID prüft s und liefert die ID. Ein Schrägstrich innerhalb der ID
// ist erlaubt, am Anfang nicht, weil das Topic sonst eine leere Ebene hätte
// ("meter//..."). + und # sind MQTT-Wildcards und deshalb verboten.
func NewMeterID(s string) (MeterID, error) {
	if s == "" || strings.HasPrefix(s, "/") || strings.ContainsAny(s, "+#") {
		return MeterID{}, ErrInvalidMeterID
	}
	return MeterID{value: s}, nil
}

// MustMeterID ist wie NewMeterID, löst bei einer ungültigen ID aber einen
// panic aus. Gedacht für Tests und feste Werte im Code.
func MustMeterID(s string) MeterID {
	id, err := NewMeterID(s)
	if err != nil {
		panic(fmt.Sprintf("MustMeterID(%q): %v", s, err))
	}
	return id
}

// IsZero meldet, ob id der Nullwert ist, also nie gesetzt wurde.
func (id MeterID) IsZero() bool { return id.value == "" }

// String macht die ID in Logs und fmt lesbar.
func (id MeterID) String() string { return id.value }

// MarshalText schreibt die ID als einfachen String, z. B. "umg-1".
func (id MeterID) MarshalText() ([]byte, error) {
	return []byte(id.value), nil
}

// UnmarshalText prüft die ID beim Einlesen. Ungültiges JSON wird so schon
// beim Parsen abgelehnt.
func (id *MeterID) UnmarshalText(b []byte) error {
	v, err := NewMeterID(string(b))
	if err != nil {
		return err
	}
	*id = v
	return nil
}
