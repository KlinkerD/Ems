package poller

import (
	"context"
	"errors"
	"testing"

	"github.com/KlinkerD/MeterService/services/modbus-meters/internal/measurement"
)

// fakeMeter schickt pro Verbindung einen Messwert und wartet dann auf Abbruch.
type fakeMeter struct {
	id      string
	value   float64
	invalid bool
}

func (f *fakeMeter) MeterID() string { return f.id }

func (f *fakeMeter) Validate() error {
	if f.invalid {
		return errors.New("ungültig")
	}
	return nil
}

func (f *fakeMeter) ReadDeviceInfo(ctx context.Context, ch chan<- measurement.DeviceInfo) error {
	select {
	case ch <- measurement.DeviceInfo{MeterID: f.id}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeMeter) ReadValues(ctx context.Context, ch chan<- []measurement.Value) error {
	select {
	case ch <- []measurement.Value{{MeterID: f.id, Type: measurement.U1, Value: f.value}}:
	case <-ctx.Done():
		return ctx.Err()
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestManagerSetReplaceRemove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	valuesCh := make(chan []measurement.Value)
	infoCh := make(chan measurement.DeviceInfo, 10)
	errCh := make(chan measurement.Error, 10)
	mg := NewManager(ctx, valuesCh, infoCh, errCh)

	if err := mg.Set(&fakeMeter{id: "m1", value: 230}); err != nil {
		t.Fatal(err)
	}
	if v := (<-valuesCh)[0].Value; v != 230 {
		t.Errorf("got %v, want 230", v)
	}

	// Neue Config mit derselben ID ersetzt die alte, ohne zweiten Eintrag.
	if err := mg.Set(&fakeMeter{id: "m1", value: 400}); err != nil {
		t.Fatal(err)
	}
	if v := (<-valuesCh)[0].Value; v != 400 {
		t.Errorf("nach Ersetzen got %v, want 400", v)
	}
	if n := len(mg.Meters()); n != 1 {
		t.Errorf("meters = %d, want 1", n)
	}

	if !mg.Remove("m1") {
		t.Error("Remove lieferte false")
	}
	if _, ok := mg.Get("m1"); ok {
		t.Error("m1 noch vorhanden")
	}
	if mg.Remove("m1") {
		t.Error("zweites Remove lieferte true")
	}

	cancel()
	mg.Wait()
	if err := mg.Set(&fakeMeter{id: "m1"}); err == nil {
		t.Error("Set nach Shutdown sollte fehlschlagen")
	}
}

func TestManagerRejectsInvalidConfig(t *testing.T) {
	mg := NewManager(context.Background(), nil, nil, nil)
	if err := mg.Set(&fakeMeter{id: "x", invalid: true}); err == nil {
		t.Error("ungültige config sollte abgelehnt werden")
	}
}
