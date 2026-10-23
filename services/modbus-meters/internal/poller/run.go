package poller

import (
	"context"
	"log/slog"
	"time"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// retryDelay ist die Wartezeit nach einem Verbindungs- oder Lesefehler.
const retryDelay = 5 * time.Second

// Run liest bei jedem Verbindungsaufbau die Geräteinfos und pollt danach die
// Messwerte. Nach Fehlern wird nach retryDelay neu verbunden. Run kehrt erst
// zurück, wenn ctx abgebrochen wird.
func Run(ctx context.Context, m Meter, valuesCh chan<- []contract.Value, infoCh chan<- contract.DeviceInfo, errCh chan<- contract.Error) {
	now := time.Now()
	for {
		err := m.ReadDeviceInfo(ctx, infoCh)
		if err == nil {
			err = m.ReadValues(ctx, valuesCh)
		}
		if ctx.Err() != nil {
			return
		}
		errCh <- contract.Error{Message: "could not connect to meter: " + err.Error(), MeterID: m.MeterID(), Timestamp: now}
		slog.Warn("meter fehler, verbinde neu", "meter", m.MeterID(), "err", err, "retryIn", retryDelay)

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return
		}
	}
}
