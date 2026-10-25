// Package meter liest einen Modbus-Zähler anhand seiner Config aus dem Vertrag
// (contracts/modbus) und lädt Configs aus Dateien.
package meter

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	contract "github.com/KlinkerD/Ems/contracts/meter"
	"github.com/KlinkerD/Ems/contracts/modbus"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/modbustcp"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/profile"
)

// Reader liest einen Zähler über eine einzige Modbus-TCP-Verbindung.
//
// ReadDeviceInfo baut die Verbindung auf und lässt sie offen, ReadValues
// nutzt sie danach weiter. Nach einem Fehler oder Abbruch wird sie
// geschlossen, der nächste Aufruf verbindet neu. Das passt zum Ablauf in
// poller.Run: pro Verbindungsrunde erst ReadDeviceInfo, dann ReadValues.
//
// Ein Reader ist nicht für gleichzeitige Aufrufe aus mehreren Goroutines
// gedacht.
type Reader struct {
	cfg  modbus.Config
	conn *modbustcp.Client
}

// NewReader erzeugt einen Reader für eine Kopie von cfg. Spätere Änderungen
// an cfg wirken sich nicht auf den Reader aus.
func NewReader(cfg *modbus.Config) *Reader {
	return &Reader{cfg: *cfg}
}

// MeterID liefert die eindeutige ID des Meters.
func (r *Reader) MeterID() contract.MeterID {
	return r.cfg.ID
}

// Validate prüft die Config des Readers.
func (r *Reader) Validate() error {
	return r.cfg.Validate()
}

// connect liefert die offene Verbindung oder baut eine neue auf.
func (r *Reader) connect(ctx context.Context) (*modbustcp.Client, error) {
	if r.conn != nil {
		return r.conn, nil
	}
	addr := r.cfg.Addr()
	c, err := modbustcp.Dial(ctx, addr, r.cfg.UnitID, r.cfg.Timeout())
	if err != nil {
		return nil, fmt.Errorf("meter %s: verbinden mit %s: %w", r.cfg.ID, addr, err)
	}
	r.conn = c
	return c, nil
}

// disconnect schließt die Verbindung, falls offen. Ein Fehler wird nur
// geloggt, da er beim Aufräumen nicht mehr sinnvoll zurückgegeben werden kann.
func (r *Reader) disconnect() {
	if r.conn == nil {
		return
	}
	if err := r.conn.Close(); err != nil {
		slog.Warn("verbindung schließen fehlgeschlagen", "meter", r.cfg.ID, "err", err)
	}
	r.conn = nil
}

// ReadDeviceInfo liest die Geräteinfos einmalig und schickt sie in ch.
// Felder ohne Register im Profil bleiben leer. Hat das Profil keine
// Geräteinfos, wird nicht verbunden. Bei Erfolg bleibt die Verbindung für
// ReadValues offen.
func (r *Reader) ReadDeviceInfo(ctx context.Context, ch chan<- contract.DeviceInfo) (err error) {
	info := contract.DeviceInfo{MeterID: r.cfg.ID}

	if len(r.cfg.Profile.DeviceInfo) > 0 {
		defer func() {
			if err != nil {
				r.disconnect()
			}
		}()
		if err := r.readDeviceInfo(ctx, &info); err != nil {
			return err
		}
	}

	select {
	case ch <- info:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Reader) readDeviceInfo(ctx context.Context, info *contract.DeviceInfo) error {
	fields, blocks := sortedBlocks(r.cfg.Profile.DeviceInfo)

	c, err := r.connect(ctx)
	if err != nil {
		return err
	}
	raw, err := c.ReadAll(ctx, blocks)
	if err != nil {
		return fmt.Errorf("meter %s: %w", r.cfg.ID, err)
	}

	for i, f := range fields {
		s, err := profile.DecodeString(r.cfg.Profile.DeviceInfo[f], raw[i])
		if err != nil {
			return fmt.Errorf("meter %s: %s dekodieren: %w", r.cfg.ID, f, err)
		}
		switch f {
		case modbus.SerialNumber:
			info.SerialNumber = s
		case modbus.Manufacturer:
			info.Manufacturer = s
		case modbus.Model:
			info.Model = s
		case modbus.Firmware:
			info.Firmware = s
		default:
			return fmt.Errorf("meter %s: unbekanntes deviceInfo-Feld %q", r.cfg.ID, f)
		}
	}
	return nil
}

// ReadValues pollt im PollingInterval alle Messwerte des Profils und schickt
// sie gesammelt in ch. Der erste Poll erfolgt sofort. Die Funktion läuft, bis
// ctx abgebrochen wird oder ein Lesefehler auftritt, und schließt dann die
// Verbindung. Das Neuverbinden übernimmt der Aufrufer.
func (r *Reader) ReadValues(ctx context.Context, ch chan<- []contract.Value) error {
	defer r.disconnect()

	if err := r.cfg.Validate(); err != nil {
		return err
	}

	// Feste Reihenfolge, damit die Werte bei jedem Poll gleich sortiert sind.
	types, blocks := sortedBlocks(r.cfg.Profile.MeterValues)

	c, err := r.connect(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(r.cfg.PollingInterval.Duration)
	defer ticker.Stop()

	for {
		raw, err := c.ReadAll(ctx, blocks)
		if err != nil {
			return fmt.Errorf("meter %s: %w", r.cfg.ID, err)
		}

		now := time.Now()
		vals := make([]contract.Value, 0, len(types))
		for i, t := range types {
			v, err := profile.DecodeNumber(r.cfg.Profile.MeterValues[t], raw[i])
			if err != nil {
				return fmt.Errorf("meter %s: %s dekodieren: %w", r.cfg.ID, t, err)
			}
			vals = append(vals, contract.Value{MeterID: r.cfg.ID, Timestamp: now, Type: t, Value: v})
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

// sortedBlocks liefert die Keys von regs sortiert und die passenden
// Modbus-Blöcke in derselben Reihenfolge.
func sortedBlocks[K cmp.Ordered, R profile.Ranger](regs map[K]R) ([]K, []modbustcp.Block) {
	keys := slices.Sorted(maps.Keys(regs))
	blocks := make([]modbustcp.Block, len(keys))
	for i, k := range keys {
		blocks[i] = profile.Block(regs[k])
	}
	return keys, blocks
}
