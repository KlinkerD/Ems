package modbus

import (
	"fmt"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// MaxRegistersPerRead ist das Limit der Modbus-Spezifikation für FC 0x03/0x04.
const MaxRegistersPerRead = 125

// Profile beschreibt herstellerabhängig, wo welcher Wert im Gerät liegt.
type Profile struct {
	DeviceType  string                             `json:"deviceType"`
	DeviceInfo  map[DeviceInfoField]StringRegister `json:"deviceInfo,omitempty"`
	MeterValues map[contract.Type]NumberRegister   `json:"meterValues"`
}

// Validate prüft das Profil beim Laden der Konfiguration,
// damit Fehler nicht erst beim ersten Poll auffallen.
func (p Profile) Validate() error {
	if p.DeviceType == "" {
		return fmt.Errorf("profile: deviceType fehlt")
	}
	if len(p.MeterValues) == 0 {
		return fmt.Errorf("profile %s: keine meterValues", p.DeviceType)
	}
	for field, reg := range p.DeviceInfo {
		if !field.IsValid() {
			return fmt.Errorf("profile %s: unbekanntes deviceInfo-Feld %q", p.DeviceType, field)
		}
		if err := reg.Validate(); err != nil {
			return fmt.Errorf("profile %s: deviceInfo %s: %w", p.DeviceType, field, err)
		}
	}
	for typ, reg := range p.MeterValues {
		if !typ.IsValid() {
			return fmt.Errorf("profile %s: unbekannter meterValue %q", p.DeviceType, typ)
		}
		if err := reg.Validate(); err != nil {
			return fmt.Errorf("profile %s: meterValue %s: %w", p.DeviceType, typ, err)
		}
	}
	return nil
}

// DeviceInfoField benennt die Felder von meter.DeviceInfo.
type DeviceInfoField string

const (
	SerialNumber DeviceInfoField = "serialNumber"
	Manufacturer DeviceInfoField = "manufacturer"
	Model        DeviceInfoField = "model"
	Firmware     DeviceInfoField = "firmware"
)

// IsValid meldet, ob f eines der bekannten Felder ist.
func (f DeviceInfoField) IsValid() bool {
	switch f {
	case SerialNumber, Manufacturer, Model, Firmware:
		return true
	default:
		return false
	}
}

// RegisterType wählt, welche Art Register gelesen wird.
type RegisterType string

const (
	HoldingRegister RegisterType = "holding" // FC 0x03
	InputRegister   RegisterType = "input"   // FC 0x04
)

// IsValid meldet, ob t ein unterstützter Registertyp ist.
func (t RegisterType) IsValid() bool {
	return t == HoldingRegister || t == InputRegister
}

// ByteOrder benennt die Reihenfolge der Bytes A B C D eines 32-Bit-Werts,
// wie es in Herstellerdokumentationen üblich ist. Für längere Werte wird
// dasselbe Muster angewendet: Bytes im Register tauschen und/oder
// Register-Reihenfolge umdrehen.
type ByteOrder string

const (
	BigEndian        ByteOrder = "ABCD" // Standard laut Modbus-Spezifikation
	LittleEndian     ByteOrder = "DCBA"
	BigEndianSwap    ByteOrder = "BADC" // Bytes im Register getauscht
	LittleEndianSwap ByteOrder = "CDAB" // Register getauscht (Word Swap)
)

// IsValid meldet, ob b eine der bekannten Byte-Reihenfolgen ist.
func (b ByteOrder) IsValid() bool {
	switch b {
	case BigEndian, LittleEndian, BigEndianSwap, LittleEndianSwap:
		return true
	default:
		return false
	}
}

// SwapsBytes meldet, ob die beiden Bytes innerhalb jedes Registers getauscht sind.
func (b ByteOrder) SwapsBytes() bool { return b == LittleEndian || b == BigEndianSwap }

// SwapsWords meldet, ob die Reihenfolge der Register umgedreht ist.
func (b ByteOrder) SwapsWords() bool { return b == LittleEndian || b == LittleEndianSwap }

// DataType legt fest, wie die Rohbytes eines Werts interpretiert werden.
// Die Länge ergibt sich nicht allein daraus, ob 2 Register float32 oder int32 sind.
type DataType string

const (
	Int16   DataType = "int16"
	Uint16  DataType = "uint16"
	Int32   DataType = "int32"
	Uint32  DataType = "uint32"
	Int64   DataType = "int64"
	Uint64  DataType = "uint64"
	Float32 DataType = "float32"
	Float64 DataType = "float64"
)

// Registers liefert die Anzahl 16-Bit-Register, die der Typ belegt, oder 0
// für einen unbekannten Typ.
func (d DataType) Registers() uint16 {
	switch d {
	case Int16, Uint16:
		return 1
	case Int32, Uint32, Float32:
		return 2
	case Int64, Uint64, Float64:
		return 4
	default:
		return 0
	}
}

// checkLength meldet einen Fehler, wenn d unbekannt ist oder nicht length Register belegt.
func (d DataType) checkLength(length uint16) error {
	n := d.Registers()
	if n == 0 {
		return fmt.Errorf("unbekannter dataType %q", d)
	}
	if n != length {
		return fmt.Errorf("dataType %s braucht length %d, nicht %d", d, n, length)
	}
	return nil
}

// Register ist der gemeinsame Teil aller Registereinträge.
type Register struct {
	Address   uint16       `json:"address"`
	Length    uint16       `json:"length"` // Anzahl 16-Bit-Register
	ByteOrder ByteOrder    `json:"byteOrder,omitzero"`
	Type      RegisterType `json:"registerType,omitzero"`
}

// Order liefert die Byte-Reihenfolge, leer bedeutet Modbus-Standard ABCD.
func (r Register) Order() ByteOrder {
	if r.ByteOrder == "" {
		return BigEndian
	}
	return r.ByteOrder
}

// RegisterType liefert den Registertyp, leer bedeutet Holding-Register.
func (r Register) RegisterType() RegisterType {
	if r.Type == "" {
		return HoldingRegister
	}
	return r.Type
}

// Range liefert Registertyp, Startadresse und Anzahl Register, also den
// Bereich, der für diesen Eintrag gelesen wird.
func (r Register) Range() (typ RegisterType, address, count uint16) {
	return r.RegisterType(), r.Address, r.Length
}

// Validate prüft Länge, Byte-Reihenfolge und Registertyp.
func (r Register) Validate() error {
	if r.Length == 0 || r.Length > MaxRegistersPerRead {
		return fmt.Errorf("length %d außerhalb 1..%d", r.Length, MaxRegistersPerRead)
	}
	if !r.Order().IsValid() {
		return fmt.Errorf("unbekannte byteOrder %q", r.ByteOrder)
	}
	if !r.RegisterType().IsValid() {
		return fmt.Errorf("unbekannter registerType %q", r.Type)
	}
	return nil
}

// StringRegister ist ein Geräteinfo-Eintrag. Ohne DataType wird er als Text
// gelesen. Mit DataType wird er als Zahl gelesen und als Dezimaltext
// geliefert, z. B. für Seriennummern, die ein Gerät als uint32 ablegt.
type StringRegister struct {
	Register
	DataType DataType `json:"dataType,omitzero"`
}

// Validate prüft das Register und, falls gesetzt, ob DataType zur Länge passt.
func (r StringRegister) Validate() error {
	if err := r.Register.Validate(); err != nil {
		return err
	}
	if r.DataType == "" {
		return nil
	}
	return r.DataType.checkLength(r.Length)
}

// NumberRegister ist ein Messwert-Eintrag, der als Zahl gelesen wird.
type NumberRegister struct {
	Register
	DataType DataType `json:"dataType"`
	// Scale wird mit dem Rohwert multipliziert, z. B. 0.1 für Werte in 0,1 V.
	// 0 bedeutet 1.
	Scale float64 `json:"scale,omitzero"`
}

// Validate prüft das Register und ob DataType zur Länge passt.
func (r NumberRegister) Validate() error {
	if err := r.Register.Validate(); err != nil {
		return err
	}
	return r.DataType.checkLength(r.Length)
}
