package profile

import (
	"errors"
	"math"
	"testing"

	"github.com/KlinkerD/Ems/contracts/modbus"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/modbustcp"
)

func TestDecodeNumberFloat32ByteOrders(t *testing.T) {
	// 230.5 als float32 = 0x43 0x66 0x80 0x00 (ABCD)
	cases := []struct {
		order modbus.ByteOrder
		raw   []byte
	}{
		{modbus.BigEndian, []byte{0x43, 0x66, 0x80, 0x00}},
		{modbus.LittleEndian, []byte{0x00, 0x80, 0x66, 0x43}},
		{modbus.BigEndianSwap, []byte{0x66, 0x43, 0x00, 0x80}},
		{modbus.LittleEndianSwap, []byte{0x80, 0x00, 0x43, 0x66}},
	}
	for _, c := range cases {
		t.Run(string(c.order), func(t *testing.T) {
			r := modbus.NumberRegister{Length: 2, ByteOrder: c.order, DataType: modbus.Float32}
			v, err := DecodeNumber(r, c.raw)
			if err != nil {
				t.Fatal(err)
			}
			if v != 230.5 {
				t.Errorf("got %v, want 230.5", v)
			}
		})
	}
}

func TestDecodeNumberScale(t *testing.T) {
	r := modbus.NumberRegister{Length: 1, DataType: modbus.Int16, Scale: 0.1}
	v, err := DecodeNumber(r, []byte{0xFF, 0xF6}) // -10
	if err != nil {
		t.Fatal(err)
	}
	if v != -1 {
		t.Errorf("got %v, want -1", v)
	}
}

func TestDecodeNumberLengthMismatch(t *testing.T) {
	r := modbus.NumberRegister{Length: 1, DataType: modbus.Float32}
	if _, err := DecodeNumber(r, []byte{0, 0}); err == nil {
		t.Error("float32 mit length 1 sollte fehlschlagen")
	}
	r = modbus.NumberRegister{Length: 2, DataType: modbus.Float32}
	if _, err := DecodeNumber(r, []byte{0, 0}); err == nil {
		t.Error("2 Bytes für 2 Register sollten fehlschlagen")
	}
}

func TestDecodeString(t *testing.T) {
	r := modbus.StringRegister{Length: 4}
	s, err := DecodeString(r, []byte("SDM630\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if s != "SDM630" {
		t.Errorf("got %q", s)
	}

	r.ByteOrder = modbus.BigEndianSwap
	s, err = DecodeString(r, []byte("DS6M03\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if s != "SDM630" {
		t.Errorf("swapped: got %q", s)
	}
}

func TestDecodeStringNumber(t *testing.T) {
	reg := func(length uint16, order modbus.ByteOrder, d modbus.DataType) modbus.StringRegister {
		return modbus.StringRegister{Length: length, ByteOrder: order, DataType: d}
	}
	cases := []struct {
		name string
		reg  modbus.StringRegister
		raw  []byte
		want string
	}{
		{"uint32 Seriennummer", reg(2, "", modbus.Uint32), []byte{0x00, 0x01, 0xE2, 0x40}, "123456"},
		{"int16 Firmware", reg(1, "", modbus.Int16), []byte{0x00, 0xFA}, "250"},
		{"int16 negativ", reg(1, "", modbus.Int16), []byte{0xFF, 0xFB}, "-5"},
		{"uint64 exakt", reg(4, "", modbus.Uint64), []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, "18446744073709551615"},
		{"uint32 CDAB", reg(2, modbus.LittleEndianSwap, modbus.Uint32), []byte{0xE2, 0x40, 0x00, 0x01}, "123456"},
		{"float32", reg(2, "", modbus.Float32), []byte{0x43, 0x66, 0x80, 0x00}, "230.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := DecodeString(c.reg, c.raw)
			if err != nil {
				t.Fatal(err)
			}
			if s != c.want {
				t.Errorf("got %q, want %q", s, c.want)
			}
		})
	}
}

func TestBlock(t *testing.T) {
	tests := []struct {
		name string
		reg  Ranger
		want modbustcp.Block
	}{
		{"Standard holding", modbus.NumberRegister{Address: 10, Length: 2}, modbustcp.Block{Type: modbus.HoldingRegister, Address: 10, Count: 2}},
		{"input", modbus.StringRegister{Address: 3, Length: 4, Type: modbus.InputRegister}, modbustcp.Block{Type: modbus.InputRegister, Address: 3, Count: 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Block(tt.reg); got != tt.want {
				t.Errorf("Block() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDecodeNumberNotFinite(t *testing.T) {
	f32 := func(v float32) []byte {
		b := math.Float32bits(v)
		return []byte{byte(b >> 24), byte(b >> 16), byte(b >> 8), byte(b)}
	}
	f64 := func(v float64) []byte {
		b := math.Float64bits(v)
		return []byte{byte(b >> 56), byte(b >> 48), byte(b >> 40), byte(b >> 32), byte(b >> 24), byte(b >> 16), byte(b >> 8), byte(b)}
	}
	reg32 := modbus.NumberRegister{Length: 2, DataType: modbus.Float32}
	reg64 := modbus.NumberRegister{Length: 4, DataType: modbus.Float64}
	tests := []struct {
		name string
		reg  modbus.NumberRegister
		raw  []byte
	}{
		{"float32 NaN", reg32, f32(float32(math.NaN()))},
		{"float32 +Inf", reg32, f32(float32(math.Inf(1)))},
		{"float32 -Inf", reg32, f32(float32(math.Inf(-1)))},
		{"float64 NaN", reg64, f64(math.NaN())},
		{"float64 +Inf", reg64, f64(math.Inf(1))},
		// Endlicher Rohwert, der erst durch die Skalierung unendlich wird.
		{"Überlauf durch scale", modbus.NumberRegister{Length: 4, DataType: modbus.Float64, Scale: 10}, f64(math.MaxFloat64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := DecodeNumber(tt.reg, tt.raw)
			if !errors.Is(err, ErrNotFinite) {
				t.Errorf("DecodeNumber() = %v, %v, want ErrNotFinite", v, err)
			}
		})
	}
}
