package meter_test

import (
	"encoding/json"
	"errors"
	"strings"
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
		{"umg+1", false},      // MQTT-Wildcard
		{"umg#1", false},      // MQTT-Wildcard
		{"umg-1/", false},     // ergäbe "meter/umg-1//config"
		{"halle//umg", false}, // leere Topic-Ebene
		{" umg-1", false},     // Leerzeichen
		{"umg 1", false},
		{"umg\t1", false},
		{"umg\x001", false}, // NUL ist in MQTT-Topics verboten
		{"umg\x7f1", false}, // Steuerzeichen
		{"umg\xff1", false}, // kein gültiges UTF-8
		{"zähler-1", true},  // Umlaute sind gültiges UTF-8
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

// TestMeterIDJSONNull prüft die Lücke, die UnmarshalText allein nicht
// schließt: Für null ruft encoding/json UnmarshalText nie auf.
func TestMeterIDJSONNull(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"null in der Liste", `{"meters":["umg-1",null]}`},
		{"Liste nur null", `{"meters":[null]}`},
		{"Schrägstrich am Ende", `{"meters":["umg-1/"]}`},
		{"doppelter Schrägstrich", `{"meters":["a//b"]}`},
		{"Leerzeichen", `{"meters":[" x "]}`},
		{"Zahl statt String", `{"meters":[1]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ids meter.MeterIDs
			err := json.Unmarshal([]byte(tt.data), &ids)
			if !errors.Is(err, meter.ErrInvalidMeterID) {
				t.Errorf("err = %v, want ErrInvalidMeterID", err)
			}
		})
	}

	var v meter.Value
	if err := json.Unmarshal([]byte(`{"meterId":null}`), &v); !errors.Is(err, meter.ErrInvalidMeterID) {
		t.Errorf("Value mit meterId null: err = %v, want ErrInvalidMeterID", err)
	}
	// Ein fehlendes Feld bleibt erlaubt und ergibt den Nullwert.
	if err := json.Unmarshal([]byte(`{"type":"U1"}`), &v); err != nil || !v.MeterID.IsZero() {
		t.Errorf("fehlendes meterId: err = %v, IsZero = %v", err, v.MeterID.IsZero())
	}
}

func TestMeterIDErrorNamesValue(t *testing.T) {
	_, err := meter.NewMeterID("a//b")
	if err == nil || !strings.Contains(err.Error(), `"a//b"`) {
		t.Errorf("err = %v, soll den Wert nennen", err)
	}
}

func TestMeterIDAsMapKey(t *testing.T) {
	var m map[meter.MeterID]int
	if err := json.Unmarshal([]byte(`{"umg-1":1}`), &m); err != nil || m[meter.MustMeterID("umg-1")] != 1 {
		t.Errorf("map: %v, err = %v", m, err)
	}
	if err := json.Unmarshal([]byte(`{"a//b":1}`), &m); !errors.Is(err, meter.ErrInvalidMeterID) {
		t.Errorf("ungültiger Map-Schlüssel: err = %v, want ErrInvalidMeterID", err)
	}
}
