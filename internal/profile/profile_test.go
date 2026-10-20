package profile

import (
	"encoding/json"
	"testing"

	"modbus/internal/measurement"
	"modbus/internal/modbustcp"
)

func TestNumberRegisterDecodeFloat32ByteOrders(t *testing.T) {
	// 230.5 als float32 = 0x43 0x66 0x80 0x00 (ABCD)
	cases := []struct {
		order ByteOrder
		raw   []byte
	}{
		{BigEndian, []byte{0x43, 0x66, 0x80, 0x00}},
		{LittleEndian, []byte{0x00, 0x80, 0x66, 0x43}},
		{BigEndianSwap, []byte{0x66, 0x43, 0x00, 0x80}},
		{LittleEndianSwap, []byte{0x80, 0x00, 0x43, 0x66}},
	}
	for _, c := range cases {
		r := NumberRegister{Register: Register{Address: 0, Length: 2, ByteOrder: c.order}, DataType: Float32}
		v, err := r.Decode(c.raw)
		if err != nil {
			t.Fatalf("%s: %v", c.order, err)
		}
		if v != 230.5 {
			t.Errorf("%s: got %v, want 230.5", c.order, v)
		}
	}
}

func TestNumberRegisterDecodeScale(t *testing.T) {
	r := NumberRegister{Register: Register{Length: 1}, DataType: Int16, Scale: 0.1}
	v, err := r.Decode([]byte{0xFF, 0xF6}) // -10
	if err != nil {
		t.Fatal(err)
	}
	if v != -1 {
		t.Errorf("got %v, want -1", v)
	}
}

func TestNumberRegisterLengthMismatch(t *testing.T) {
	r := NumberRegister{Register: Register{Length: 1}, DataType: Float32}
	if _, err := r.Decode([]byte{0, 0}); err == nil {
		t.Error("float32 mit length 1 sollte fehlschlagen")
	}
}

func TestStringRegisterDecode(t *testing.T) {
	r := StringRegister{Register{Length: 4}}
	s, err := r.Decode([]byte("SDM630\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if s != "SDM630" {
		t.Errorf("got %q", s)
	}

	r.ByteOrder = BigEndianSwap
	s, err = r.Decode([]byte("DS6M03\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if s != "SDM630" {
		t.Errorf("swapped: got %q", s)
	}
}

func TestProfileJSON(t *testing.T) {
	data := `{
		"deviceType": "Eastron SDM630",
		"deviceInfo": {
			"serialNumber": {"address": 64512, "length": 2, "registerType": "holding"}
		},
		"meterValues": {
			"U1": {"address": 0, "length": 2, "byteOrder": "ABCD", "registerType": "input", "dataType": "float32"},
			"ActiveEnergyImported": {"address": 72, "length": 2, "registerType": "input", "dataType": "float32", "scale": 1000}
		}
	}`
	var p Profile
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	u1 := p.MeterValues[measurement.U1]
	if u1.RegisterType() != modbustcp.InputRegister || u1.DataType != Float32 || u1.Length != 2 {
		t.Errorf("U1 falsch geparst: %+v", u1)
	}
	if p.DeviceInfo[SerialNumber].RegisterType() != modbustcp.HoldingRegister {
		t.Error("serialNumber sollte holding sein")
	}
}

func TestProfileValidateRejectsUnknownKeys(t *testing.T) {
	p := Profile{
		DeviceType:  "x",
		MeterValues: map[measurement.Type]NumberRegister{"Foo": {Register: Register{Length: 1}, DataType: Uint16}},
	}
	if err := p.Validate(); err == nil {
		t.Error("unbekannter meterValue sollte fehlschlagen")
	}
}
