package modbustcp

import (
	"context"
	"testing"

	"github.com/KlinkerD/Ems/contracts/modbus"
)

// TestFunctionCode prüft die Zuordnung der Registertypen aus dem Vertrag zu
// den Modbus-Funktionscodes.
func TestFunctionCode(t *testing.T) {
	tests := []struct {
		typ     modbus.RegisterType
		want    byte
		wantErr bool
	}{
		{modbus.HoldingRegister, 0x03, false},
		{modbus.InputRegister, 0x04, false},
		{"coil", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			got, err := functionCode(tt.typ)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("functionCode(%q) = 0x%02x, %v, want 0x%02x, wantErr %v", tt.typ, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

// TestReadRegistersLimit prüft das Lese-Limit aus dem Vertrag. Die Prüfung
// passiert vor jedem Netzwerkzugriff, deshalb reicht ein Client ohne Verbindung.
func TestReadRegistersLimit(t *testing.T) {
	var c Client
	for _, count := range []uint16{0, modbus.MaxRegistersPerRead + 1} {
		if _, err := c.readRegisters(context.Background(), modbus.HoldingRegister, 0, count); err == nil {
			t.Errorf("count %d sollte abgelehnt werden", count)
		}
	}
}
