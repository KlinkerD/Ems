package physics_test

import (
	"math"
	"testing"

	"github.com/KlinkerD/Ems/contracts/physics"
)

const eps = 1e-9

func near(a, b float64) bool { return math.Abs(a-b) < eps }

func TestApparentFromPQ(t *testing.T) {
	tests := []struct {
		name string
		p    physics.Watt
		q    physics.Var
		want physics.VoltAmpere
	}{
		{"3-4-5", 3, 4, 5},
		{"negatives P", -3, 4, 5},
		{"negatives Q", 3, -4, 5},
		{"null", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := physics.ApparentFromPQ(tt.p, tt.q); !near(float64(got), float64(tt.want)) {
				t.Errorf("ApparentFromPQ(%v, %v) = %v, want %v", tt.p, tt.q, got, tt.want)
			}
		})
	}
}

func TestApparentFromUI(t *testing.T) {
	if got := physics.ApparentFromUI(230, 10); !near(float64(got), 2300) {
		t.Errorf("ApparentFromUI(230, 10) = %v, want 2300", got)
	}
}

// result ist das gemeinsame Ergebnis aller Funktionen mit (Wert, ok).
type result struct {
	value float64
	ok    bool
}

func check(t *testing.T, got, want result) {
	t.Helper()
	if got.ok != want.ok {
		t.Fatalf("ok = %v, want %v", got.ok, want.ok)
	}
	if got.ok && !near(got.value, want.value) {
		t.Errorf("value = %v, want %v", got.value, want.value)
	}
}

func TestCurrentFromSU(t *testing.T) {
	tests := []struct {
		name string
		s    physics.VoltAmpere
		u    physics.Volt
		want result
	}{
		{"normal", 2300, 230, result{10, true}},
		{"U null", 2300, 0, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.CurrentFromSU(tt.s, tt.u)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestVoltageFromSI(t *testing.T) {
	tests := []struct {
		name string
		s    physics.VoltAmpere
		i    physics.Ampere
		want result
	}{
		{"normal", 2300, 10, result{230, true}},
		{"I null", 2300, 0, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.VoltageFromSI(tt.s, tt.i)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestReactiveFromSP(t *testing.T) {
	tests := []struct {
		name string
		s    physics.VoltAmpere
		p    physics.Watt
		want result
	}{
		{"3-4-5", 5, 3, result{4, true}},
		{"negatives P gibt Betrag", 5, -3, result{4, true}},
		{"P gleich S", 5, 5, result{0, true}},
		{"P größer S", 5, 6, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.ReactiveFromSP(tt.s, tt.p)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestActiveFromSQ(t *testing.T) {
	tests := []struct {
		name string
		s    physics.VoltAmpere
		q    physics.Var
		want result
	}{
		{"3-4-5", 5, 4, result{3, true}},
		{"negatives Q gibt Betrag", 5, -4, result{3, true}},
		{"Q größer S", 5, 6, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.ActiveFromSQ(tt.s, tt.q)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestActiveFromSCos(t *testing.T) {
	tests := []struct {
		name string
		s    physics.VoltAmpere
		cos  float64
		want result
	}{
		{"normal", 1000, 0.8, result{800, true}},
		{"negativ", 1000, -0.8, result{-800, true}},
		{"cos 1", 1000, 1, result{1000, true}},
		{"cos größer 1", 1000, 1.1, result{}},
		{"cos kleiner -1", 1000, -1.1, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.ActiveFromSCos(tt.s, tt.cos)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestReactiveFromSCos(t *testing.T) {
	tests := []struct {
		name string
		s    physics.VoltAmpere
		cos  float64
		want result
	}{
		{"normal", 1000, 0.8, result{600, true}},
		{"negativ gibt Betrag", 1000, -0.8, result{600, true}},
		{"cos 1", 1000, 1, result{0, true}},
		{"cos 0", 1000, 0, result{1000, true}},
		{"cos größer 1", 1000, 1.1, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.ReactiveFromSCos(tt.s, tt.cos)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestApparentFromPCos(t *testing.T) {
	tests := []struct {
		name string
		p    physics.Watt
		cos  float64
		want result
	}{
		{"normal", 800, 0.8, result{1000, true}},
		{"Einspeisung", -800, -0.8, result{1000, true}},
		{"Vorzeichen gemischt", -800, 0.8, result{1000, true}},
		{"cos 0", 800, 0, result{}},
		{"cos größer 1", 800, 1.1, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.ApparentFromPCos(tt.p, tt.cos)
			check(t, result{float64(v), ok}, tt.want)
		})
	}
}

func TestPowerFactor(t *testing.T) {
	tests := []struct {
		name string
		p    physics.Watt
		s    physics.VoltAmpere
		want result
	}{
		{"Bezug", 800, 1000, result{0.8, true}},
		{"Einspeisung", -800, 1000, result{-0.8, true}},
		{"S null", 800, 0, result{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := physics.PowerFactor(tt.p, tt.s)
			check(t, result{v, ok}, tt.want)
		})
	}
}
