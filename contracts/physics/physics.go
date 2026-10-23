// Package physics beschreibt nur die Formeln zwischen elektrischen Größen.
// Ein false bedeutet: mathematisch nicht bestimmbar. Plausibilität prüft
// der aufrufende Service.
package physics

import "math"

type (
	Volt       float64
	Ampere     float64
	Watt       float64
	Var        float64
	VoltAmpere float64
)

// S = √(P² + Q²)
func ApparentFromPQ(p Watt, q Var) VoltAmpere {
	return VoltAmpere(math.Hypot(float64(p), float64(q)))
}

// S = U · I
func ApparentFromUI(u Volt, i Ampere) VoltAmpere {
	return VoltAmpere(float64(u) * float64(i))
}

// I = S / U
func CurrentFromSU(s VoltAmpere, u Volt) (Ampere, bool) {
	if u == 0 {
		return 0, false
	}
	return Ampere(float64(s) / float64(u)), true
}

// U = S / I
func VoltageFromSI(s VoltAmpere, i Ampere) (Volt, bool) {
	if i == 0 {
		return 0, false
	}
	return Volt(float64(s) / float64(i)), true
}

// |Q| = √(S² − P²). Das Vorzeichen von Q ist so nicht bestimmbar.
func ReactiveFromSP(s VoltAmpere, p Watt) (Var, bool) {
	d := float64(s)*float64(s) - float64(p)*float64(p)
	if d < 0 {
		return 0, false
	}
	return Var(math.Sqrt(d)), true
}

// |P| = √(S² − Q²). Das Vorzeichen von P ist so nicht bestimmbar.
func ActiveFromSQ(s VoltAmpere, q Var) (Watt, bool) {
	d := float64(s)*float64(s) - float64(q)*float64(q)
	if d < 0 {
		return 0, false
	}
	return Watt(math.Sqrt(d)), true
}

// P = S · cos φ
func ActiveFromSCos(s VoltAmpere, cos float64) (Watt, bool) {
	if math.Abs(cos) > 1 {
		return 0, false
	}
	return Watt(float64(s) * cos), true
}

// |Q| = S · √(1 − cos²φ)
func ReactiveFromSCos(s VoltAmpere, cos float64) (Var, bool) {
	if math.Abs(cos) > 1 {
		return 0, false
	}
	return Var(float64(s) * math.Sqrt(1-cos*cos)), true
}

// S = |P| / |cos φ|
func ApparentFromPCos(p Watt, cos float64) (VoltAmpere, bool) {
	if cos == 0 || math.Abs(cos) > 1 {
		return 0, false
	}
	return VoltAmpere(math.Abs(float64(p)) / math.Abs(cos)), true
}

// λ = P / S
func PowerFactor(p Watt, s VoltAmpere) (float64, bool) {
	if s == 0 {
		return 0, false
	}
	return float64(p) / float64(s), true
}
