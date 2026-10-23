// Package electricity bildet die elektrischen Größen einer Messung ab.
// Fehlende Größen werden über die Methoden mit package physics berechnet.
package electricity

import (
	"time"

	"github.com/KlinkerD/Ems/contracts/physics"
)

type Source string

const (
	Measured Source = "measured"
	Derived  Source = "derived"
)

type Quantity struct {
	Value       float64 `json:"value"`
	Source      Source  `json:"source"`
	SignUnknown bool    `json:"signUnknown,omitempty"`
}

type Phase struct {
	Index int       `json:"index"`
	U     *Quantity `json:"u,omitempty"`
	P     *Quantity `json:"p,omitempty"`
	Q     *Quantity `json:"q,omitempty"`
	I     *Quantity `json:"i,omitempty"` // nur, wenn nicht aus U, P, Q berechenbar
}

type Measurement struct {
	MeterID   string    `json:"meterId"`
	Timestamp time.Time `json:"timestamp"`
	Phases    []Phase   `json:"phases"`
}

// Apparent liefert S: √(P² + Q²), sonst U · I.
// Sie liest nur Felder und ruft keine andere Methode auf. Deshalb dürfen
// alle anderen Methoden sie nutzen, ohne dass eine Schleife entsteht.
func (ph Phase) Apparent() (Quantity, bool) {
	if ph.P != nil && ph.Q != nil {
		s := physics.ApparentFromPQ(physics.Watt(ph.P.Value), physics.Var(ph.Q.Value))
		return derived(float64(s), false), true
	}
	if ph.U != nil && ph.I != nil {
		s := physics.ApparentFromUI(physics.Volt(ph.U.Value), physics.Ampere(ph.I.Value))
		return derived(float64(s), false), true
	}
	return Quantity{}, false
}

// Voltage liefert U: gespeichert, sonst S / I.
func (ph Phase) Voltage() (Quantity, bool) {
	if ph.U != nil {
		return *ph.U, true
	}
	if ph.I == nil {
		return Quantity{}, false
	}
	s, ok := ph.Apparent()
	if !ok {
		return Quantity{}, false
	}
	u, ok := physics.VoltageFromSI(physics.VoltAmpere(s.Value), physics.Ampere(ph.I.Value))
	if !ok {
		return Quantity{}, false
	}
	return derived(float64(u), false), true
}

// Current liefert I: gespeichert, sonst S / U.
func (ph Phase) Current() (Quantity, bool) {
	if ph.I != nil {
		return *ph.I, true
	}
	if ph.U == nil {
		return Quantity{}, false
	}
	s, ok := ph.Apparent()
	if !ok {
		return Quantity{}, false
	}
	i, ok := physics.CurrentFromSU(physics.VoltAmpere(s.Value), physics.Volt(ph.U.Value))
	if !ok {
		return Quantity{}, false
	}
	return derived(float64(i), false), true
}

// Active liefert P: gespeichert, sonst |P| = √(S² − Q²) ohne Vorzeichen.
func (ph Phase) Active() (Quantity, bool) {
	if ph.P != nil {
		return *ph.P, true
	}
	if ph.Q == nil {
		return Quantity{}, false
	}
	s, ok := ph.Apparent() // P fehlt, also nur über U · I möglich
	if !ok {
		return Quantity{}, false
	}
	p, ok := physics.ActiveFromSQ(physics.VoltAmpere(s.Value), physics.Var(ph.Q.Value))
	if !ok {
		return Quantity{}, false
	}
	return derived(float64(p), true), true
}

// Reactive liefert Q: gespeichert, sonst |Q| = √(S² − P²) ohne Vorzeichen.
func (ph Phase) Reactive() (Quantity, bool) {
	if ph.Q != nil {
		return *ph.Q, true
	}
	if ph.P == nil {
		return Quantity{}, false
	}
	s, ok := ph.Apparent() // Q fehlt, also nur über U · I möglich
	if !ok {
		return Quantity{}, false
	}
	q, ok := physics.ReactiveFromSP(physics.VoltAmpere(s.Value), physics.Watt(ph.P.Value))
	if !ok {
		return Quantity{}, false
	}
	return derived(float64(q), true), true
}

// PowerFactor liefert λ = P / S. Das Vorzeichen folgt P.
func (ph Phase) PowerFactor() (Quantity, bool) {
	p, ok := ph.Active()
	if !ok {
		return Quantity{}, false
	}
	s, ok := ph.Apparent()
	if !ok {
		return Quantity{}, false
	}
	pf, ok := physics.PowerFactor(physics.Watt(p.Value), physics.VoltAmpere(s.Value))
	if !ok {
		return Quantity{}, false
	}
	return derived(pf, p.SignUnknown), true
}

func derived(v float64, signUnknown bool) Quantity {
	return Quantity{Value: v, Source: Derived, SignUnknown: signUnknown}
}
