package meter_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/KlinkerD/Ems/contracts/meter"
)

func TestTopicForMeter(t *testing.T) {
	tests := []struct {
		name  string
		topic meter.Topic
		id    string
		want  meter.Topic
	}{
		{"mit Platzhalter", meter.TopicValues, "umg-1", "meter/umg-1/values"},
		{"ohne Platzhalter", meter.TopicMeterIDs, "umg-1", "config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.topic.ForMeter(tt.id); got != tt.want {
				t.Errorf("ForMeter(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestTypeUnit(t *testing.T) {
	tests := []struct {
		typ  meter.Type
		want meter.Unit
	}{
		{meter.U1, meter.UnitV},
		{meter.I3, meter.UnitA},
		{meter.P2, meter.UnitW},
		{meter.Q1, meter.UnitVar},
		{meter.S3, meter.UnitVA},
		{meter.ActiveEnergyExported, meter.UnitWh},
		{"Unbekannt", meter.UnitNone},
	}
	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			if got := tt.typ.Unit(); got != tt.want {
				t.Errorf("Unit() = %q, want %q", got, tt.want)
			}
			if got, want := tt.typ.IsValid(), tt.want != meter.UnitNone; got != want {
				t.Errorf("IsValid() = %v, want %v", got, want)
			}
		})
	}
}

// TestValueJSON sichert die Feldnamen ab, auf die sich Empfänger verlassen.
func TestValueJSON(t *testing.T) {
	v := meter.Value{
		MeterID:   "umg-1",
		Timestamp: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Type:      meter.U1,
		Value:     230.5,
	}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"meterId":"umg-1","timestamp":"2026-10-23T12:00:00Z","type":"U1","value":230.5}`
	if string(got) != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}
