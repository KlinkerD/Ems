// Package profile beschreibt herstellerabhängig, in welchen Modbus-Registern
// welche Werte liegen, und dekodiert deren Rohbytes.
package profile

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"modbus/internal/measurement"
	"modbus/internal/modbustcp"
)

// Profile beschreibt herstellerabhängig, wo welcher Wert im Gerät liegt.
type Profile struct {
	DeviceType  string                              `json:"deviceType"`
	DeviceInfo  map[DeviceInfoField]StringRegister  `json:"deviceInfo,omitempty"`
	MeterValues map[measurement.Type]NumberRegister `json:"meterValues"`
}

// DeviceInfoField benennt die Felder von measurement.DeviceInfo.
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

func (b ByteOrder) swapBytes() bool { return b == LittleEndian || b == BigEndianSwap }
func (b ByteOrder) swapWords() bool { return b == LittleEndian || b == LittleEndianSwap }

// IsValid meldet, ob b eine der bekannten Byte-Reihenfolgen ist.
func (b ByteOrder) IsValid() bool {
	switch b {
	case BigEndian, LittleEndian, BigEndianSwap, LittleEndianSwap:
		return true
	default:
		return false
	}
}

// DataType legt fest, wie die Rohbytes eines Messwerts interpretiert werden.
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

// registers liefert die Anzahl 16-Bit-Register, die der Typ belegt.
func (d DataType) registers() uint16 {
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

// Register ist der gemeinsame Teil aller Registereinträge.
type Register struct {
	Address   uint16                 `json:"address"`
	Length    uint16                 `json:"length"` // Anzahl 16-Bit-Register
	ByteOrder ByteOrder              `json:"byteOrder,omitzero"`
	Type      modbustcp.RegisterType `json:"registerType,omitzero"`
}

// order liefert die Byte-Reihenfolge, leer bedeutet Modbus-Standard ABCD.
func (r Register) order() ByteOrder {
	if r.ByteOrder == "" {
		return BigEndian
	}
	return r.ByteOrder
}

// RegisterType liefert den Registertyp, leer bedeutet Holding-Register.
func (r Register) RegisterType() modbustcp.RegisterType {
	if r.Type == "" {
		return modbustcp.HoldingRegister
	}
	return r.Type
}

// Block liefert den Registerbereich, der für diesen Eintrag gelesen wird.
func (r Register) Block() modbustcp.Block {
	return modbustcp.Block{Type: r.RegisterType(), Address: r.Address, Count: r.Length}
}

func (r Register) validate() error {
	if r.Length == 0 || r.Length > modbustcp.MaxRegistersPerRead {
		return fmt.Errorf("length %d außerhalb 1..%d", r.Length, modbustcp.MaxRegistersPerRead)
	}
	if !r.order().IsValid() {
		return fmt.Errorf("unbekannte byteOrder %q", r.ByteOrder)
	}
	if !r.RegisterType().IsValid() {
		return fmt.Errorf("unbekannter registerType %q", r.Type)
	}
	return nil
}

// normalize bringt die Rohbytes in Big-Endian-Reihenfolge (ABCD).
func (r Register) normalize(raw []byte) ([]byte, error) {
	if len(raw) != int(r.Length)*2 {
		return nil, fmt.Errorf("erwartet %d Bytes, erhalten %d", r.Length*2, len(raw))
	}
	b := make([]byte, len(raw))
	copy(b, raw)
	o := r.order()
	if o.swapWords() {
		for i, j := 0, len(b)-2; i < j; i, j = i+2, j-2 {
			b[i], b[i+1], b[j], b[j+1] = b[j], b[j+1], b[i], b[i+1]
		}
	}
	if o.swapBytes() {
		for i := 0; i < len(b); i += 2 {
			b[i], b[i+1] = b[i+1], b[i]
		}
	}
	return b, nil
}

// StringRegister ist ein Geräteinfo-Eintrag, der als Text dekodiert wird.
type StringRegister struct {
	Register
}

// Decode wandelt die Rohbytes in einen String; Null-Bytes und Leerzeichen
// am Rand werden entfernt, da Geräte Strings damit auffüllen.
func (r StringRegister) Decode(raw []byte) (string, error) {
	b, err := r.normalize(raw)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Trim(string(b), "\x00")), nil
}

// NumberRegister ist ein Messwert-Eintrag, der als float64 dekodiert wird.
type NumberRegister struct {
	Register
	DataType DataType `json:"dataType"`
	// Scale wird mit dem Rohwert multipliziert, z. B. 0.1 für Werte in 0,1 V.
	// 0 bedeutet 1.
	Scale float64 `json:"scale,omitzero"`
}

func (r NumberRegister) validate() error {
	if err := r.Register.validate(); err != nil {
		return err
	}
	n := r.DataType.registers()
	if n == 0 {
		return fmt.Errorf("unbekannter dataType %q", r.DataType)
	}
	if n != r.Length {
		return fmt.Errorf("dataType %s braucht length %d, nicht %d", r.DataType, n, r.Length)
	}
	return nil
}

// Decode wandelt die Rohbytes in einen float64 inklusive Skalierung.
func (r NumberRegister) Decode(raw []byte) (float64, error) {
	if err := r.validate(); err != nil {
		return 0, err
	}
	b, err := r.normalize(raw)
	if err != nil {
		return 0, err
	}

	var v float64
	switch r.DataType {
	case Int16:
		v = float64(int16(binary.BigEndian.Uint16(b)))
	case Uint16:
		v = float64(binary.BigEndian.Uint16(b))
	case Int32:
		v = float64(int32(binary.BigEndian.Uint32(b)))
	case Uint32:
		v = float64(binary.BigEndian.Uint32(b))
	case Int64:
		v = float64(int64(binary.BigEndian.Uint64(b)))
	case Uint64:
		v = float64(binary.BigEndian.Uint64(b))
	case Float32:
		v = float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case Float64:
		v = math.Float64frombits(binary.BigEndian.Uint64(b))
	}

	if r.Scale != 0 {
		v *= r.Scale
	}
	return v, nil
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
		if err := reg.validate(); err != nil {
			return fmt.Errorf("profile %s: deviceInfo %s: %w", p.DeviceType, field, err)
		}
	}
	for typ, reg := range p.MeterValues {
		if !typ.IsValid() {
			return fmt.Errorf("profile %s: unbekannter meterValue %q", p.DeviceType, typ)
		}
		if err := reg.validate(); err != nil {
			return fmt.Errorf("profile %s: meterValue %s: %w", p.DeviceType, typ, err)
		}
	}
	return nil
}
