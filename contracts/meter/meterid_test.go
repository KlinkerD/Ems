package meter_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/KlinkerD/Ems/contracts/meter"
)

func TestNewMeterID(t *testing.T) {
	tests := []struct {
		in    string
		valid bool
	}{
		{"umg-1", true},
		{"halle-3/umg-1", true}, // Schrägstrich innerhalb ist erlaubt
		{"", false},
		{"/umg-1", false}, // ergäbe "meter//umg-1/config"
		{"/", false},
		{"umg+1", false}, // MQTT-Wildcard
		{"umg#1", false}, // MQTT-Wildcard
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			id, err := meter.NewMeterID(tt.in)
			if tt.valid {
				if err != nil {
					t.Fatalf("NewMeterID(%q) unerwarteter Fehler: %v", tt.in, err)
				}
				if id.String() != tt.in {
					t.Errorf("String() = %q, want %q", id, tt.in)
				}
				return
			}
			if !errors.Is(err, meter.ErrInvalidMeterID) {
				t.Errorf("NewMeterID(%q) err = %v, want ErrInvalidMeterID", tt.in, err)
			}
		})
	}
}

func TestMeterIDJSON(t *testing.T) {
	var got struct {
		ID meter.MeterID `json:"id"`
	}
	if err := json.Unmarshal([]byte(`{"id":"halle-3/umg-1"}`), &got); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"id":"halle-3/umg-1"}` {
		t.Errorf("json = %s", b)
	}
	if err := json.Unmarshal([]byte(`{"id":"/umg-1"}`), &got); err == nil {
		t.Error(`"/umg-1" sollte abgelehnt werden`)
	}
}

func TestMeterIDZeroAndMust(t *testing.T) {
	var zero meter.MeterID
	if !zero.IsZero() {
		t.Error("Nullwert sollte IsZero sein")
	}
	if meter.MustMeterID("umg-1").IsZero() {
		t.Error("gesetzte ID sollte nicht IsZero sein")
	}
	defer func() {
		if recover() == nil {
			t.Error("MustMeterID(\"\") sollte panic auslösen")
		}
	}()
	meter.MustMeterID("")
}

func TestMeterIDsJSON(t *testing.T) {
	var ids meter.MeterIDs
	if err := json.Unmarshal([]byte(`{"version":1,"meters":["umg-1","umg-2"]}`), &ids); err != nil {
		t.Fatal(err)
	}
	if len(ids.Meters) != 2 || ids.Meters[1] != meter.MustMeterID("umg-2") {
		t.Errorf("meters = %v", ids.Meters)
	}
	if err := json.Unmarshal([]byte(`{"meters":["umg-1",""]}`), &ids); err == nil {
		t.Error("leere ID in der Liste sollte abgelehnt werden")
	}
}
