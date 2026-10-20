package values

import "time"

// ValueType ist ein String, damit er in JSON, MQTT und Logs lesbar bleibt
// und das Umsortieren der Konstanten keine gespeicherten Daten verfälscht.
type ValueType string

const (
	I1 ValueType = "I1"
	I2 ValueType = "I2"
	I3 ValueType = "I3"

	U1 ValueType = "U1"
	U2 ValueType = "U2"
	U3 ValueType = "U3"

	P1 ValueType = "P1"
	P2 ValueType = "P2"
	P3 ValueType = "P3"

	Q1 ValueType = "Q1"
	Q2 ValueType = "Q2"
	Q3 ValueType = "Q3"

	S1 ValueType = "S1"
	S2 ValueType = "S2"
	S3 ValueType = "S3"

	ActiveEnergyImported ValueType = "ActiveEnergyImported"
	ActiveEnergyExported ValueType = "ActiveEnergyExported"
)

type Unit string

const (
	UnitNone Unit = ""
	UnitA    Unit = "A"
	UnitV    Unit = "V"
	UnitW    Unit = "W"
	UnitVar  Unit = "var"
	UnitVA   Unit = "VA"
	UnitWh   Unit = "Wh"
)

// Unit liefert die Einheit, in der ein Wert dieses Typs vorliegt.
// Ein leerer Rückgabewert bedeutet unbekannter Typ.
func (v ValueType) Unit() Unit {
	switch v {
	case I1, I2, I3:
		return UnitA
	case U1, U2, U3:
		return UnitV
	case P1, P2, P3:
		return UnitW
	case Q1, Q2, Q3:
		return UnitVar
	case S1, S2, S3:
		return UnitVA
	case ActiveEnergyImported, ActiveEnergyExported:
		return UnitWh
	default:
		return UnitNone
	}
}

// IsValid nutzt Unit(), damit neue Typen nur an einer Stelle gepflegt werden.
func (v ValueType) IsValid() bool {
	return v.Unit() != UnitNone
}

// MeterValue ist ein einzelner Messwert, so wie er weiterverschickt wird.
type MeterValue struct {
	MeterID   string    `json:"meterId"`
	Timestamp time.Time `json:"timestamp"`
	Type      ValueType `json:"type"`
	Value     float64   `json:"value"`
}

// DeviceInfo wird einmal beim Verbindungsaufbau gelesen.
type DeviceInfo struct {
	MeterID      string `json:"meterId"`
	SerialNumber string `json:"serialNumber,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Firmware     string `json:"firmware,omitempty"`
}
