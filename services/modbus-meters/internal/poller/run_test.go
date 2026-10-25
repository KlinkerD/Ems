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

// blockedMeter schlägt fehl und meldet, dass Run gleich auf errCh senden will.
type blockedMeter struct {
	fakeMeter
	called chan struct{}
}

func (b *blockedMeter) ReadDeviceInfo(context.Context, chan<- contract.DeviceInfo) error {
	select {
	case b.called <- struct{}{}:
	default:
	}
	return errors.New("keine verbindung")
}

// TestRunReturnsWhenErrChNotRead prüft, dass Run nach dem Abbruch zurückkehrt,
// auch wenn niemand mehr auf errCh liest. Vorher blieb Run im Senden hängen und
// mit ihm Manager.Wait beim Beenden.
func TestRunReturnsWhenErrChNotRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &blockedMeter{fakeMeter: fakeMeter{id: contract.MustMeterID("m1")}, called: make(chan struct{}, 1)}
	errCh := make(chan contract.Error) // niemand liest
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, m, nil, nil, errCh)
	}()

	<-m.called // Run hat den Fehler und will ihn gleich senden
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run kehrt nach Abbruch nicht zurück, blockiert auf errCh")
	}
}

// TestManagerWaitWithoutReader prüft dasselbe über den Manager: Beenden mit
// einem fehlerhaften Meter darf nicht hängen, wenn niemand errCh liest.
func TestManagerWaitWithoutReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &blockedMeter{fakeMeter: fakeMeter{id: contract.MustMeterID("m1")}, called: make(chan struct{}, 1)}
	mg := NewManager(ctx, nil, nil, make(chan contract.Error))
	if err := mg.Set(m); err != nil {
		t.Fatal(err)
	}
	<-m.called
	cancel()
	done := make(chan struct{})
	go func() { mg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Manager.Wait hängt")
	}
}
