// Package profile liest Register anhand eines Profils aus dem Vertrag
// (contracts/modbus) und dekodiert deren Rohbytes. Die Profil-Typen selbst
// und ihre Prüfung liegen im Vertrag, hier steckt nur die Logik.
package profile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/KlinkerD/Ems/contracts/modbus"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/modbustcp"
)

// Ranger ist jeder Registereintrag aus dem Vertrag. modbus.NumberRegister und
// modbus.StringRegister erfüllen es über das eingebettete modbus.Register.
type Ranger interface {
	Range() (typ modbus.RegisterType, address, count uint16)
}

// Block liefert den Registerbereich, der für r gelesen wird.
func Block(r Ranger) modbustcp.Block {
	typ, addr, count := r.Range()
	return modbustcp.Block{Type: typ, Address: addr, Count: count}
}

// ErrNotFinite meldet einen Messwert, der NaN oder ±Inf ist. Viele Zähler
// liefern NaN für "Wert gerade nicht verfügbar". JSON kann solche Werte nicht
// darstellen, deshalb dürfen sie nicht weitergeschickt werden.
var ErrNotFinite = errors.New("wert ist NaN oder unendlich")

// DecodeNumber wandelt die Rohbytes in einen float64 inklusive Skalierung.
// Ist das Ergebnis NaN oder ±Inf, liefert es einen Fehler mit ErrNotFinite.
func DecodeNumber(r modbus.NumberRegister, raw []byte) (float64, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	b, err := normalize(r.Register, raw)
	if err != nil {
		return 0, err
	}

	var v float64
	switch r.DataType {
	case modbus.Int16:
		v = float64(int16(binary.BigEndian.Uint16(b)))
	case modbus.Uint16:
		v = float64(binary.BigEndian.Uint16(b))
	case modbus.Int32:
		v = float64(int32(binary.BigEndian.Uint32(b)))
	case modbus.Uint32:
		v = float64(binary.BigEndian.Uint32(b))
	case modbus.Int64:
		v = float64(int64(binary.BigEndian.Uint64(b)))
	case modbus.Uint64:
		v = float64(binary.BigEndian.Uint64(b))
	case modbus.Float32:
		v = float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case modbus.Float64:
		v = math.Float64frombits(binary.BigEndian.Uint64(b))
	}

	if r.Scale != 0 {
		v *= r.Scale
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("%w: %v", ErrNotFinite, v)
	}
	return v, nil
}

// DecodeString wandelt die Rohbytes in einen String. Bei Text werden
// Null-Bytes und Leerzeichen am Rand entfernt, da Geräte Strings damit
// auffüllen. Mit DataType wird die Zahl als Dezimaltext geliefert.
func DecodeString(r modbus.StringRegister, raw []byte) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	b, err := normalize(r.Register, raw)
	if err != nil {
		return "", err
	}
	if r.DataType == "" {
		return strings.TrimSpace(strings.Trim(string(b), "\x00")), nil
	}
	return format(r.DataType, b), nil
}

// normalize bringt die Rohbytes in Big-Endian-Reihenfolge (ABCD).
func normalize(r modbus.Register, raw []byte) ([]byte, error) {
	if len(raw) != int(r.Length)*2 {
		return nil, fmt.Errorf("erwartet %d Bytes, erhalten %d", r.Length*2, len(raw))
	}
	b := make([]byte, len(raw))
	copy(b, raw)
	o := r.Order()
	if o.SwapsWords() {
		for i, j := 0, len(b)-2; i < j; i, j = i+2, j-2 {
			b[i], b[i+1], b[j], b[j+1] = b[j], b[j+1], b[i], b[i+1]
		}
	}
	if o.SwapsBytes() {
		for i := 0; i < len(b); i += 2 {
			b[i], b[i+1] = b[i+1], b[i]
		}
	}
	return b, nil
}

// format liefert die Big-Endian-Bytes b als Dezimaltext. Ganzzahlen werden
// exakt formatiert, nicht über float64, damit lange Seriennummern stimmen.
func format(d modbus.DataType, b []byte) string {
	switch d {
	case modbus.Int16:
		return strconv.FormatInt(int64(int16(binary.BigEndian.Uint16(b))), 10)
	case modbus.Uint16:
		return strconv.FormatUint(uint64(binary.BigEndian.Uint16(b)), 10)
	case modbus.Int32:
		return strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(b))), 10)
	case modbus.Uint32:
		return strconv.FormatUint(uint64(binary.BigEndian.Uint32(b)), 10)
	case modbus.Int64:
		return strconv.FormatInt(int64(binary.BigEndian.Uint64(b)), 10)
	case modbus.Uint64:
		return strconv.FormatUint(binary.BigEndian.Uint64(b), 10)
	case modbus.Float32:
		return strconv.FormatFloat(float64(math.Float32frombits(binary.BigEndian.Uint32(b))), 'f', -1, 32)
	case modbus.Float64:
		return strconv.FormatFloat(math.Float64frombits(binary.BigEndian.Uint64(b)), 'f', -1, 64)
	default:
		return ""
	}
}
