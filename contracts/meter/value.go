package meter

// Type ist ein String, damit er in JSON, MQTT und Logs lesbar bleibt
// und das Umsortieren der Konstanten keine gespeicherten Daten verfälscht.
type Type string

const (
	I1 Type = "I1"
	I2 Type = "I2"
	I3 Type = "I3"

	U1 Type = "U1"
	U2 Type = "U2"
	U3 Type = "U3"

	P1 Type = "P1"
	P2 Type = "P2"
	P3 Type = "P3"

	Q1 Type = "Q1"
	Q2 Type = "Q2"
	Q3 Type = "Q3"

	S1 Type = "S1"
	S2 Type = "S2"
	S3 Type = "S3"

	COS_PHI_1 Type = "COS_PHI_1"
	COS_PHI_2 Type = "COS_PHI_2"
	COS_PHI_3 Type = "COS_PHI_3"

	ActiveEnergyImported Type = "ActiveEnergyImported"
	ActiveEnergyExported Type = "ActiveEnergyExported"
)

// Unit ist die physikalische Einheit eines Messwerts.
type Unit string

const (
	UnitNone Unit = ""
	UnitA    Unit = "A"
	UnitV    Unit = "V"
	UnitW    Unit = "W"
	UnitVar  Unit = "var"
	UnitVA   Unit = "VA"
	UnitWh   Unit = "Wh"
	// UnitDimensionless gilt für Verhältnisgrößen wie cos φ (SI-Einheit 1).
	UnitDimensionless Unit = "1"
)

// Unit liefert die Einheit, in der ein Wert dieses Typs vorliegt.
// Ein leerer Rückgabewert bedeutet unbekannter Typ.
func (t Type) Unit() Unit {
	switch t {
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
	case COS_PHI_1, COS_PHI_2, COS_PHI_3:
		return UnitDimensionless
	default:
		return UnitNone
	}
}

// IsValid nutzt Unit(), damit neue Typen nur an einer Stelle gepflegt werden.
func (t Type) IsValid() bool {
	return t.Unit() != UnitNone
}
