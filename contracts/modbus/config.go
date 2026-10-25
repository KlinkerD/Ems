// Package modbus beschreibt die Config eines Modbus-TCP-Zählers, so wie sie
// auf meter.TopicConfig liegt: Verbindungsdaten und ein Profil, das sagt, in
// welchen Registern welche Werte stehen.
//
// Das Paket enthält nur Daten und deren Prüfung. Das Lesen und Dekodieren der
// Register übernimmt der Service, der die Config nutzt.
package modbus

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// DefaultPort ist der Standard-Port für Modbus TCP.
const DefaultPort = 502

// DefaultTimeout gilt pro Anfrage, wenn in der Config kein Timeout gesetzt ist.
const DefaultTimeout = time.Second

// Config ist die Konfiguration eines einzelnen Zählers. Jeder Zähler hat eine
// eigene Datei bzw. Nachricht, damit Configs einzeln ausgetauscht werden können.
type Config struct {
	Version         int              `json:"version"`
	ID              contract.MeterID `json:"id"`
	Host            string           `json:"host"`
	Port            uint16           `json:"port"`
	UnitID          uint8            `json:"unitId"`
	PollingInterval Duration         `json:"pollingInterval"`
	ReadTimeout     Duration         `json:"timeout,omitzero"`
	Profile         Profile          `json:"profile"`
}

// Parse dekodiert und prüft eine einzelne Config, z. B. aus einer Datei oder
// einer MQTT-Nachricht.
func Parse(data []byte) (*Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate prüft die Config, damit Fehler beim Laden auffallen
// und nicht erst beim ersten Poll.
func (c *Config) Validate() error {
	if c.ID.IsZero() {
		return errors.New("id fehlt")
	}
	if c.Host == "" {
		return fmt.Errorf("meter %s: host fehlt", c.ID)
	}
	if c.PollingInterval.Duration <= 0 {
		return fmt.Errorf("meter %s: pollingInterval muss > 0 sein", c.ID)
	}
	if err := c.Profile.Validate(); err != nil {
		return fmt.Errorf("meter %s: %w", c.ID, err)
	}
	return nil
}

// Timeout liefert den Timeout pro Anfrage, ohne Angabe DefaultTimeout.
func (c *Config) Timeout() time.Duration {
	return cmp.Or(c.ReadTimeout.Duration, DefaultTimeout)
}

// Addr liefert "host:port", ohne Port-Angabe mit DefaultPort.
func (c *Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(int(cmp.Or(c.Port, DefaultPort))))
}

// Duration erlaubt "5s" oder "20ms" in JSON statt Nanosekunden.
type Duration struct {
	time.Duration
}

// UnmarshalText parst Angaben im Format von time.ParseDuration.
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// MarshalText schreibt die Dauer im Format von time.Duration.String.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}
