package meter

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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

// MeterID identifiziert einen Zähler. Eine gesetzte ID entsteht nur über
// NewMeterID, MustMeterID oder das Einlesen aus JSON und ist dann immer
// gültig. Der Nullwert (MeterID{}) bedeutet "nicht gesetzt", z. B. wenn ein
// Feld im JSON fehlt; mit IsZero prüfen. In JSON erscheint sie als einfacher
// String.
type MeterID struct {
	value string
}

// ErrInvalidMeterID wird von NewMeterID und beim Einlesen aus JSON geliefert,
// wenn eine ID nicht als Teil eines MQTT-Topics taugt.
var ErrInvalidMeterID = errors.New("ungültige meter-id")

// NewMeterID prüft s und liefert die ID. Die ID wird ein Teil des Topics
// "meter/{id}/...". Deshalb gilt:
//   - Schrägstriche innerhalb sind erlaubt ("halle-3/umg-1"), aber nicht am
//     Anfang, am Ende oder doppelt, weil das Topic sonst eine leere Ebene hätte.
//   - + und # sind MQTT-Wildcards und verboten.
//   - Leerzeichen, Steuerzeichen (auch NUL) und ungültiges UTF-8 sind
//     verboten, MQTT erlaubt in Topics kein NUL und verlangt UTF-8.
func NewMeterID(s string) (MeterID, error) {
	if s == "" ||
		strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") || strings.Contains(s, "//") ||
		strings.ContainsAny(s, "+#") ||
		!utf8.ValidString(s) ||
		strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return MeterID{}, fmt.Errorf("%w: %q", ErrInvalidMeterID, s)
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

// UnmarshalJSON prüft die ID beim Einlesen aus JSON. Anders als UnmarshalText
// wird es auch für null aufgerufen; null wird abgelehnt, damit z. B. eine
// Meter-Liste nie eine leere ID enthält.
func (id *MeterID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return fmt.Errorf("%w: null", ErrInvalidMeterID)
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidMeterID, err)
	}
	return id.UnmarshalText([]byte(s))
}

// UnmarshalText prüft die ID beim Einlesen, z. B. als Schlüssel einer
// JSON-Map. Ungültige Werte werden so schon beim Parsen abgelehnt.
func (id *MeterID) UnmarshalText(b []byte) error {
	v, err := NewMeterID(string(b))
	if err != nil {
		return err
	}
	*id = v
	return nil
}
