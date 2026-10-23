package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/KlinkerD/MeterService/services/modbus-meters/internal/measurement"
)

// retryDelay ist die Wartezeit nach einem Verbindungs- oder Lesefehler.
const retryDelay = 5 * time.Second

// Run liest bei jedem Verbindungsaufbau die Geräteinfos und pollt danach die
// Messwerte. Nach Fehlern wird nach retryDelay neu verbunden. Run kehrt erst
// zurück, wenn ctx abgebrochen wird.
func Run(ctx context.Context, m Meter, valuesCh chan<- []measurement.Value, infoCh chan<- measurement.DeviceInfo, errCh chan<- measurement.Error) {
	now := time.Now()
	for {
		err := m.ReadDeviceInfo(ctx, infoCh)
		if err == nil {
			err = m.ReadValues(ctx, valuesCh)
		}
		if ctx.Err() != nil {
			return
		}
		errCh <- measurement.Error{Message: "could not connect to meter: " + err.Error(), MeterID: m.MeterID(), Timestamp: now}
		slog.Warn("meter fehler, verbinde neu", "meter", m.MeterID(), "err", err, "retryIn", retryDelay)

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return
		}
	}
}
