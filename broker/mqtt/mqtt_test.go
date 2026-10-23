package mqtt_test

import (
	"testing"

	"github.com/KlinkerD/Ems/broker/mqtt"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     mqtt.Config
		wantErr bool
	}{
		{"gültig", mqtt.Config{BrokerURL: "tcp://localhost:1883", ClientID: "test"}, false},
		{"mit Zugangsdaten", mqtt.Config{BrokerURL: "tcp://localhost:1883", ClientID: "test", Username: "u", Password: "p"}, false},
		{"ohne BrokerURL", mqtt.Config{ClientID: "test"}, true},
		{"ohne ClientID", mqtt.Config{BrokerURL: "tcp://localhost:1883"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := mqtt.New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && h == nil {
				t.Fatal("New() = nil, want Handler")
			}
		})
	}
}
