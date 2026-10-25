package modbus_test

import (
	"encoding/json"
	"testing"
	"time"

	contract "github.com/KlinkerD/Ems/contracts/meter"
	"github.com/KlinkerD/Ems/contracts/modbus"
)

const validConfig = `{
	"version": 1,
	"id": "umg-1",
	"host": "192.168.1.50",
	"unitId": 1,
	"pollingInterval": "1s",
	"profile": {
		"deviceType": "Janitza UMG 96 RM",
		"meterValues": {
			"U1": {"address": 19000, "length": 2, "dataType": "float32"}
		}
	}
}`

func TestParse(t *testing.T) {
	c, err := modbus.Parse([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != contract.MustMeterID("umg-1") || c.PollingInterval.Duration != time.Second {
		t.Errorf("config falsch geparst: %+v", c)
	}
	if got, want := c.Addr(), "192.168.1.50:502"; got != want {
		t.Errorf("Addr() = %q, want %q (Standard-Port)", got, want)
	}
	if got := c.Timeout(); got != modbus.DefaultTimeout {
		t.Errorf("Timeout() = %v, want %v", got, modbus.DefaultTimeout)
	}
}

func TestConfigValidate(t *testing.T) {
	valid := func() modbus.Config {
		c, err := modbus.Parse([]byte(validConfig))
		if err != nil {
			t.Fatal(err)
		}
		return *c
	}
	tests := []struct {
		name   string
		change func(*modbus.Config)
	}{
		{"ohne id", func(c *modbus.Config) { c.ID = contract.MeterID{} }},
		{"ohne host", func(c *modbus.Config) { c.Host = "" }},
		{"ohne pollingInterval", func(c *modbus.Config) { c.PollingInterval = modbus.Duration{} }},
		{"ohne meterValues", func(c *modbus.Config) { c.Profile.MeterValues = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.change(&c)
			if err := c.Validate(); err == nil {
				t.Error("Validate() = nil, want Fehler")
			}
		})
	}
}

func TestParseRejectsInvalidMeterID(t *testing.T) {
	data := []byte(`{"id": "+", "host": "h", "pollingInterval": "1s", "profile": {}}`)
	if _, err := modbus.Parse(data); err == nil {
		t.Error("ungültige meter-id sollte fehlschlagen")
	}
}

func TestProfileValidateDeviceInfoDataType(t *testing.T) {
	base := func(r modbus.StringRegister) modbus.Profile {
		return modbus.Profile{
			DeviceType: "x",
			DeviceInfo: map[modbus.DeviceInfoField]modbus.StringRegister{modbus.SerialNumber: r},
			MeterValues: map[contract.Type]modbus.NumberRegister{
				contract.U1: {Length: 2, DataType: modbus.Float32},
			},
		}
	}
	tests := []struct {
		name    string
		reg     modbus.StringRegister
		wantErr bool
	}{
		{"uint32 mit length 1", modbus.StringRegister{Length: 1, DataType: modbus.Uint32}, true},
		{"unbekannter dataType", modbus.StringRegister{Length: 2, DataType: "text"}, true},
		{"uint32 mit length 2", modbus.StringRegister{Length: 2, DataType: modbus.Uint32}, false},
		{"Text ohne dataType", modbus.StringRegister{Length: 4}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := base(tt.reg).Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
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
	var p modbus.Profile
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	u1 := p.MeterValues[contract.U1]
	if u1.RegisterType() != modbus.InputRegister || u1.DataType != modbus.Float32 || u1.Length != 2 {
		t.Errorf("U1 falsch geparst: %+v", u1)
	}
	if p.DeviceInfo[modbus.SerialNumber].RegisterType() != modbus.HoldingRegister {
		t.Error("serialNumber sollte holding sein")
	}
}

func TestProfileValidateRejects(t *testing.T) {
	tests := []struct {
		name string
		reg  modbus.NumberRegister
		typ  contract.Type
	}{
		{"unbekannter meterValue", modbus.NumberRegister{Length: 1, DataType: modbus.Uint16}, "Foo"},
		{"length 0", modbus.NumberRegister{Length: 0, DataType: modbus.Uint16}, contract.U1},
		{"zu lang", modbus.NumberRegister{Length: modbus.MaxRegistersPerRead + 1, DataType: modbus.Uint16}, contract.U1},
		{"unbekannte byteOrder", modbus.NumberRegister{Length: 1, ByteOrder: "XYZW", DataType: modbus.Uint16}, contract.U1},
		{"unbekannter registerType", modbus.NumberRegister{Length: 1, Type: "coil", DataType: modbus.Uint16}, contract.U1},
		{"dataType passt nicht zur Länge", modbus.NumberRegister{Length: 1, DataType: modbus.Float32}, contract.U1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := modbus.Profile{DeviceType: "x", MeterValues: map[contract.Type]modbus.NumberRegister{tt.typ: tt.reg}}
			if err := p.Validate(); err == nil {
				t.Error("Validate() = nil, want Fehler")
			}
		})
	}
}

func TestRegisterDefaults(t *testing.T) {
	var r modbus.Register
	if r.Order() != modbus.BigEndian {
		t.Errorf("Order() = %q, want ABCD", r.Order())
	}
	typ, addr, count := modbus.Register{Address: 7, Length: 2}.Range()
	if typ != modbus.HoldingRegister || addr != 7 || count != 2 {
		t.Errorf("Range() = %v %d %d", typ, addr, count)
	}
}
