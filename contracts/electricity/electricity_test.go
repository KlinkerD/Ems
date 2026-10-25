package electricity_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/KlinkerD/Ems/contracts/electricity"
	"github.com/KlinkerD/Ems/contracts/meter"
)

const eps = 1e-9

func m(v float64) *electricity.Quantity {
	return &electricity.Quantity{Value: v, Source: electricity.Measured}
}

// want beschreibt das erwartete Ergebnis einer Methode. Der Nullwert want{}
// bedeutet: nicht bestimmbar (ok == false).
type want struct {
	ok          bool
	value       float64
	source      electricity.Source
	signUnknown bool
}

func measured(v float64) want { return want{ok: true, value: v, source: electricity.Measured} }
func derived(v float64) want  { return want{ok: true, value: v, source: electricity.Derived} }
func derivedAbs(v float64) want {
	return want{ok: true, value: v, source: electricity.Derived, signUnknown: true}
}

func check(t *testing.T, method string, got electricity.Quantity, ok bool, w want) {
	t.Helper()
	if ok != w.ok {
		t.Errorf("%s: ok = %v, want %v", method, ok, w.ok)
		return
	}
	if !ok {
		return
	}
	if math.Abs(got.Value-w.value) > eps {
		t.Errorf("%s: value = %v, want %v", method, got.Value, w.value)
	}
	if got.Source != w.source {
		t.Errorf("%s: source = %q, want %q", method, got.Source, w.source)
	}
	if got.SignUnknown != w.signUnknown {
		t.Errorf("%s: signUnknown = %v, want %v", method, got.SignUnknown, w.signUnknown)
	}
}

// TestPhaseMethods deckt jede Kombination gespeicherter Felder ab und prüft
// für jede alle Methoden auf Wert, Quelle und SignUnknown.
func TestPhaseMethods(t *testing.T) {
	const (
		u = 230.0
		i = 10.0
		p = 2000.0
		q = 1000.0
	)
	sPQ := math.Hypot(p, q) // S aus P und Q
	sUI := u * i            // S aus U und I

	tests := []struct {
		name     string
		phase    electricity.Phase
		voltage  want
		current  want
		active   want
		reactive want
		apparent want
		pf       want
	}{
		{
			name:     "alles gemessen, S kommt aus P und Q",
			phase:    electricity.Phase{U: m(u), I: m(i), P: m(p), Q: m(q)},
			voltage:  measured(u),
			current:  measured(i),
			active:   measured(p),
			reactive: measured(q),
			apparent: derived(sPQ),
			pf:       derived(p / sPQ),
		},
		{
			name:     "U, P, Q: I wird berechnet",
			phase:    electricity.Phase{U: m(u), P: m(p), Q: m(q)},
			voltage:  measured(u),
			current:  derived(sPQ / u),
			active:   measured(p),
			reactive: measured(q),
			apparent: derived(sPQ),
			pf:       derived(p / sPQ),
		},
		{
			name:     "P, Q, I: U wird berechnet",
			phase:    electricity.Phase{I: m(i), P: m(p), Q: m(q)},
			voltage:  derived(sPQ / i),
			current:  measured(i),
			active:   measured(p),
			reactive: measured(q),
			apparent: derived(sPQ),
			pf:       derived(p / sPQ),
		},
		{
			name:     "U, I: nur S bestimmbar",
			phase:    electricity.Phase{U: m(u), I: m(i)},
			voltage:  measured(u),
			current:  measured(i),
			apparent: derived(sUI),
		},
		{
			name:     "U, I, P: Q nur als Betrag",
			phase:    electricity.Phase{U: m(u), I: m(i), P: m(p)},
			voltage:  measured(u),
			current:  measured(i),
			active:   measured(p),
			reactive: derivedAbs(math.Sqrt(sUI*sUI - p*p)),
			apparent: derived(sUI),
			pf:       derived(p / sUI),
		},
		{
			name:     "U, I, Q: P nur als Betrag, Leistungsfaktor auch",
			phase:    electricity.Phase{U: m(u), I: m(i), Q: m(q)},
			voltage:  measured(u),
			current:  measured(i),
			active:   derivedAbs(math.Sqrt(sUI*sUI - q*q)),
			reactive: measured(q),
			apparent: derived(sUI),
			pf:       derivedAbs(math.Sqrt(sUI*sUI-q*q) / sUI),
		},
		{
			name:     "U, I, Q mit Q größer S: P nicht bestimmbar",
			phase:    electricity.Phase{U: m(u), I: m(i), Q: m(3000)},
			voltage:  measured(u),
			current:  measured(i),
			reactive: measured(3000),
			apparent: derived(sUI),
		},
		{
			name:     "P, Q: weder U noch I bestimmbar",
			phase:    electricity.Phase{P: m(p), Q: m(q)},
			active:   measured(p),
			reactive: measured(q),
			apparent: derived(sPQ),
			pf:       derived(p / sPQ),
		},
		{
			name:    "nur U",
			phase:   electricity.Phase{U: m(u)},
			voltage: measured(u),
		},
		{
			name:    "nur I",
			phase:   electricity.Phase{I: m(i)},
			current: measured(i),
		},
		{
			name:  "leer",
			phase: electricity.Phase{},
		},
		{
			name:     "U null: I nicht bestimmbar",
			phase:    electricity.Phase{U: m(0), P: m(0), Q: m(0)},
			voltage:  measured(0),
			active:   measured(0),
			reactive: measured(0),
			apparent: derived(0),
		},
		{
			name:     "Einspeisung: Leistungsfaktor negativ",
			phase:    electricity.Phase{U: m(u), P: m(-p), Q: m(q)},
			voltage:  measured(u),
			current:  derived(sPQ / u),
			active:   measured(-p),
			reactive: measured(q),
			apparent: derived(sPQ),
			pf:       derived(-p / sPQ),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := tt.phase.Voltage()
			check(t, "Voltage", v, ok, tt.voltage)
			v, ok = tt.phase.Current()
			check(t, "Current", v, ok, tt.current)
			v, ok = tt.phase.Active()
			check(t, "Active", v, ok, tt.active)
			v, ok = tt.phase.Reactive()
			check(t, "Reactive", v, ok, tt.reactive)
			v, ok = tt.phase.Apparent()
			check(t, "Apparent", v, ok, tt.apparent)
			v, ok = tt.phase.PowerFactor()
			check(t, "PowerFactor", v, ok, tt.pf)
		})
	}
}

// TestMeasurementJSON sichert die Feldnamen ab, auf die sich Empfänger verlassen.
func TestMeasurementJSON(t *testing.T) {
	msg := electricity.Measurement{
		MeterID:   meter.MustMeterID("umg-1"),
		Timestamp: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Phases: []electricity.Phase{{
			Index: 1,
			U:     m(230),
			Q:     &electricity.Quantity{Value: 100, Source: electricity.Derived, SignUnknown: true},
		}},
	}
	got, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"meterId":"umg-1","timestamp":"2026-10-23T12:00:00Z","phases":[{"index":1,` +
		`"u":{"value":230,"source":"measured"},` +
		`"q":{"value":100,"source":"derived","signUnknown":true}}]}`
	if string(got) != want {
		t.Errorf("json =\n%s\nwant\n%s", got, want)
	}
}
