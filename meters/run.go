package meters

import (
	"context"
	"log/slog"
	"time"

	"modbus/meters/values"
)

// retryDelay ist die Wartezeit nach einem Verbindungs- oder Lesefehler.
const retryDelay = 5 * time.Second

// Run liest bei jedem Verbindungsaufbau die Geräteinfos und pollt danach die
// Messwerte. Nach Fehlern wird nach retryDelay neu verbunden. Run kehrt erst
// zurück, wenn ctx abgebrochen wird.
func Run(ctx context.Context, m PollableMeter, valuesCh chan<- []values.MeterValue, infoCh chan<- values.DeviceInfo) {
	for {
		err := m.ReadDeviceInfo(ctx, infoCh)
		if err == nil {
			err = m.ReadValues(ctx, valuesCh)
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("meter fehler, verbinde neu", "meter", m.MeterID(), "err", err, "retryIn", retryDelay)

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return
		}
	}
}
