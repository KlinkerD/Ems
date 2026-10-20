package meters

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"maps"
	"net"
	"slices"
	"strconv"
	"time"

	"modbus/meters/values"
)

// defaultTimeout gilt pro Anfrage, wenn im Meter kein Timeout gesetzt ist.
const defaultTimeout = time.Second

// ModbusMeter ist die Konfiguration eines einzelnen Zählers. Jeder Zähler hat
// eine eigene Datei bzw. Nachricht, damit Configs einzeln ausgetauscht werden können.
type ModbusMeter struct {
	Version         int      `json:"version"`
	ID              string   `json:"id"`
	Host            string   `json:"host"`
	Port            uint16   `json:"port"`
	UnitID          uint8    `json:"unitId"`
	PollingInterval Duration `json:"pollingInterval"`
	ReadTimeout     Duration `json:"timeout,omitzero"`
	Profile         Profile  `json:"profile"`
}

func (m *ModbusMeter) dial(ctx context.Context) (*tcpClient, error) {
	timeout := m.Timeout().Duration
	addr := net.JoinHostPort(m.Host, strconv.Itoa(int(cmp.Or(m.Port, 502))))
	c, err := dialTCP(ctx, addr, m.UnitID, timeout)
	if err != nil {
		return nil, fmt.Errorf("meter %s: verbinden mit %s: %w", m.ID, addr, err)
	}
	return c, nil
}

// readValues pollt im PollingInterval alle Messwerte des Profils und schickt
// sie gesammelt in ch. Der erste Poll erfolgt sofort. Die Funktion läuft, bis
// ctx abgebrochen wird oder ein Lesefehler auftritt; das Neuverbinden
// übernimmt der Aufrufer.
func (m *ModbusMeter) ReadValues(ctx context.Context, ch chan<- []values.MeterValue) error {
	if m.PollingInterval.Duration <= 0 {
		return fmt.Errorf("meter %s: pollingInterval muss > 0 sein", m.ID)
	}
	if len(m.Profile.MeterValues) == 0 {
		return fmt.Errorf("meter %s: profil enthält keine meterValues", m.ID)
	}

	// Feste Reihenfolge, damit die Werte bei jedem Poll gleich sortiert sind.
	types := slices.Sorted(maps.Keys(m.Profile.MeterValues))
	regs := make([]Register, len(types))
	for i, t := range types {
		regs[i] = m.Profile.MeterValues[t].Register
	}

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer func(c *tcpClient) {
		err := c.Close()
		if err != nil {
			log.Printf("meter %s: tcpClient.Close: %v", m.ID, err)
		}
	}(c)

	ticker := time.NewTicker(m.PollingInterval.Duration)
	defer ticker.Stop()

	for {
		raw, err := c.readAll(ctx, regs)
		if err != nil {
			return fmt.Errorf("meter %s: %w", m.ID, err)
		}

		now := time.Now()
		vals := make([]values.MeterValue, 0, len(types))
		for i, t := range types {
			v, err := m.Profile.MeterValues[t].Decode(raw[i])
			if err != nil {
				return fmt.Errorf("meter %s: %s dekodieren: %w", m.ID, t, err)
			}
			vals = append(vals, values.MeterValue{MeterID: m.ID, Timestamp: now, Type: t, Value: v})
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

// readDeviceInfo liest die Geräteinfos einmalig und schickt sie in ch.
// Felder ohne Register im Profil bleiben leer.
func (m *ModbusMeter) ReadDeviceInfo(ctx context.Context, ch chan<- values.DeviceInfo) error {
	info := values.DeviceInfo{MeterID: m.ID}

	if len(m.Profile.DeviceInfo) > 0 {
		fields := slices.Sorted(maps.Keys(m.Profile.DeviceInfo))
		regs := make([]Register, len(fields))
		for i, f := range fields {
			regs[i] = m.Profile.DeviceInfo[f].Register
		}

		c, err := m.dial(ctx)
		if err != nil {
			return err
		}
		defer c.Close()

		raw, err := c.readAll(ctx, regs)
		if err != nil {
			return fmt.Errorf("meter %s: %w", m.ID, err)
		}

		for i, f := range fields {
			s, err := m.Profile.DeviceInfo[f].Decode(raw[i])
			if err != nil {
				return fmt.Errorf("meter %s: %s dekodieren: %w", m.ID, f, err)
			}
			switch f {
			case SerialNumber:
				info.SerialNumber = s
			case Manufacturer:
				info.Manufacturer = s
			case Model:
				info.Model = s
			case Firmware:
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

func (m *ModbusMeter) Interval() Duration {
	return m.PollingInterval
}
func (m *ModbusMeter) Timeout() Duration {
	return Duration{cmp.Or(m.ReadTimeout.Duration, defaultTimeout)}
}

func (m *ModbusMeter) MeterID() string {
	return m.ID
}
