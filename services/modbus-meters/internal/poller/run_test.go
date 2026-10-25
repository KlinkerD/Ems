package poller

import (
	"context"
	"errors"
	"testing"
	"time"

	contract "github.com/KlinkerD/Ems/contracts/meter"
)

// failingMeter schlägt bei jedem Verbindungsaufbau fehl.
type failingMeter struct{ fakeMeter }

func (f *failingMeter) ReadDeviceInfo(context.Context, chan<- contract.DeviceInfo) error {
	return errors.New("keine verbindung")
}

// TestRunErrorTimestamp prüft, dass jede Fehlermeldung den Zeitpunkt ihres
// Fehlers trägt und nicht den Startzeitpunkt von Run.
func TestRunErrorTimestamp(t *testing.T) {
	old := retryDelay
	retryDelay = 20 * time.Millisecond
	t.Cleanup(func() { retryDelay = old })

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan contract.Error)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, &failingMeter{fakeMeter{id: contract.MustMeterID("m1")}}, nil, nil, errCh)
	}()

	first := <-errCh
	second := <-errCh
	cancel()
	<-done

	if m1 := contract.MustMeterID("m1"); first.MeterID != m1 || second.MeterID != m1 {
		t.Errorf("meterIDs = %q, %q, want m1", first.MeterID, second.MeterID)
	}
	if !second.Timestamp.After(first.Timestamp) {
		t.Errorf("zweiter Fehler %v liegt nicht nach erstem %v", second.Timestamp, first.Timestamp)
	}
}
