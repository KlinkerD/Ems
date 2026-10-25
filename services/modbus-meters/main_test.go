package main

import (
	"testing"

	"github.com/KlinkerD/Ems/broker"
	"github.com/KlinkerD/Ems/contracts"
	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// TestHandleMeterConfigInvalid prüft, dass eine ungültige Config nicht zum
// Absturz führt und der Status unter der ID aus dem Topic gemeldet wird.
func TestHandleMeterConfigInvalid(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"kein json", "{"},
		{"host fehlt", `{"id":"umg-1","pollingInterval":"1s"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCh := make(chan contract.Status, 1)
			msg := broker.Message{Topic: contract.TopicConfig.ForMeter(contract.MustMeterID("umg-1")), Payload: []byte(tt.payload)}

			// Manager wird bei ungültiger Config nicht gebraucht.
			handleMeterConfig(msg, nil, statusCh)

			got := <-statusCh
			if got.MeterID != contract.MustMeterID("umg-1") || got.Message != "Invalid Config" {
				t.Errorf("status = %+v, want umg-1 / Invalid Config", got)
			}
		})
	}
}

func TestMeterIDFromConfigTopic(t *testing.T) {
	tests := []struct {
		topic contracts.Topic
		want  string
	}{
		{"meter/umg-1/config", "umg-1"},
		{"meter/a/b/config", "a/b"},
		{"meter//config", ""},
		{"meter/um+g/config", ""}, // ungültige ID
		{"meter/umg-1/values", ""},
		{"config", ""},
	}
	for _, tt := range tests {
		if got := meterIDFromConfigTopic(tt.topic).String(); got != tt.want {
			t.Errorf("meterIDFromConfigTopic(%q) = %q, want %q", tt.topic, got, tt.want)
		}
	}
}
