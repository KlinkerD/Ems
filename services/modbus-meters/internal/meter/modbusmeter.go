// Package meter enthält die Konfiguration eines Modbus-Zählers und liest
// anhand seines Profils Messwerte und Geräteinfos aus.
package meter

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/KlinkerD/Ems/services/modbus-meters/internal/measurement"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/modbustcp"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/profile"
)

// defaultTimeout gilt pro Anfrage, wenn im Meter kein Timeout gesetzt ist.
const defaultTimeout = time.Second

// defaultPort ist der Standard-Port für Modbus TCP.
const defaultPort = 502

// ModbusMeter ist die Konfiguration eines einzelnen Zählers. Jeder Zähler hat
// eine eigene Datei bzw. Nachricht, damit Configs einzeln ausgetauscht werden können.
type ModbusMeter struct {
	Version         int             `json:"version"`
	ID              string          `json:"id"`
	Host            string          `json:"host"`
	Port            uint16          `json:"port"`
	UnitID          uint8           `json:"unitId"`
	PollingInterval Duration        `json:"pollingInterval"`
	ReadTimeout     Duration        `json:"timeout,omitzero"`
	Profile         profile.Profile `json:"profile"`
}

func (m *ModbusMeter) dial(ctx context.Context) (*modbustcp.Client, error) {
	addr := net.JoinHostPort(m.Host, strconv.Itoa(int(cmp.Or(m.Port, defaultPort))))
	c, err := modbustcp.Dial(ctx, addr, m.UnitID, m.Timeout().Duration)
	if err != nil {
		return nil, fmt.Errorf("meter %s: verbinden mit %s: %w", m.ID, addr, err)
	}
	return c, nil
}

// closeConn schließt c und loggt einen Fehler, da er beim Aufräumen nicht
// mehr sinnvoll zurückgegeben werden kann.
func (m *ModbusMeter) closeConn(c *modbustcp.Client) {
	if err := c.Close(); err != nil {
		slog.Warn("verbindung schließen fehlgeschlagen", "meter", m.ID, "err", err)
	}
}

// ReadValues pollt im PollingInterval alle Messwerte des Profils und schickt
// sie gesammelt in ch. Der erste Poll erfolgt sofort. Die Funktion läuft, bis
// ctx abgebrochen wird oder ein Lesefehler auftritt; das Neuverbinden
// übernimmt der Aufrufer.
func (m *ModbusMeter) ReadValues(ctx context.Context, ch chan<- []measurement.Value) error {
	if err := m.Validate(); err != nil {
		return err
	}

	// Feste Reihenfolge, damit die Werte bei jedem Poll gleich sortiert sind.
	types := slices.Sorted(maps.Keys(m.Profile.MeterValues))
	blocks := make([]modbustcp.Block, len(types))
	for i, t := range types {
		blocks[i] = m.Profile.MeterValues[t].Block()
	}

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer m.closeConn(c)

	ticker := time.NewTicker(m.PollingInterval.Duration)
	defer ticker.Stop()

	for {
		raw, err := c.ReadAll(ctx, blocks)
		if err != nil {
			return fmt.Errorf("meter %s: %w", m.ID, err)
		}

		now := time.Now()
		vals := make([]measurement.Value, 0, len(types))
		for i, t := range types {
			v, err := m.Profile.MeterValues[t].Decode(raw[i])
			if err != nil {
				return fmt.Errorf("meter %s: %s dekodieren: %w", m.ID, t, err)
			}
			vals = append(vals, measurement.Value{MeterID: m.ID, Timestamp: now, Type: t, Value: v})
		}

		select {
		case ch <- vals:
		case <-ctx.Done():
			return ctx.Err()
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ReadDeviceInfo liest die Geräteinfos einmalig und schickt sie in ch.
// Felder ohne Register im Profil bleiben leer.
func (m *ModbusMeter) ReadDeviceInfo(ctx context.Context, ch chan<- measurement.DeviceInfo) error {
	info := measurement.DeviceInfo{MeterID: m.ID}

	if len(m.Profile.DeviceInfo) > 0 {
		fields := slices.Sorted(maps.Keys(m.Profile.DeviceInfo))
		blocks := make([]modbustcp.Block, len(fields))
		for i, f := range fields {
			blocks[i] = m.Profile.DeviceInfo[f].Block()
		}

		c, err := m.dial(ctx)
		if err != nil {
			return err
		}
		defer m.closeConn(c)

		raw, err := c.ReadAll(ctx, blocks)
		if err != nil {
			return fmt.Errorf("meter %s: %w", m.ID, err)
		}

		for i, f := range fields {
			s, err := m.Profile.DeviceInfo[f].Decode(raw[i])
			if err != nil {
				return fmt.Errorf("meter %s: %s dekodieren: %w", m.ID, f, err)
			}
			switch f {
			case profile.SerialNumber:
				info.SerialNumber = s
			case profile.Manufacturer:
				info.Manufacturer = s
			case profile.Model:
				info.Model = s
			case profile.Firmware:
				info.Firmware = s
			default:
				return fmt.Errorf("meter %s: unbekanntes deviceInfo-Feld %q", m.ID, f)
			}
		}
	}

	select {
	case ch <- info:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Interval liefert das Poll-Intervall.
func (m *ModbusMeter) Interval() Duration {
	return m.PollingInterval
}

// Timeout liefert den Timeout pro Anfrage, ohne Angabe defaultTimeout.
func (m *ModbusMeter) Timeout() Duration {
	return Duration{cmp.Or(m.ReadTimeout.Duration, defaultTimeout)}
}

// MeterID liefert die eindeutige ID des Meters.
func (m *ModbusMeter) MeterID() string {
	return m.ID
}
