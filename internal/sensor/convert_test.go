package sensor

import (
	"math"
	"testing"
)

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestFDEC(t *testing.T) {
	v, s := fDEC(0, UnitMetric)
	if v != 0 || s != "0" {
		t.Errorf("fDEC(0) = (%f, %s), want (0, '0')", v, s)
	}
	v, s = fDEC(255, UnitMetric)
	if v != 255 || s != "255" {
		t.Errorf("fDEC(255) = (%f, %s), want (255, '255')", v, s)
	}
}

func TestFERPM(t *testing.T) {
	// 31.25 * 0 = 0 rpm
	v, s := fERPM(0, UnitMetric)
	if v != 0 {
		t.Errorf("fERPM(0) = %f, want 0", v)
	}
	if s != "0rpm" {
		t.Errorf("fERPM(0) string = %s, want '0rpm'", s)
	}

	// 31.25 * 128 = 4000 rpm
	v, _ = fERPM(128, UnitMetric)
	if !approxEqual(v, 4000, 1) {
		t.Errorf("fERPM(128) = %f, want ~4000", v)
	}

	// 31.25 * 255 = 7968.75 rpm
	v, _ = fERPM(255, UnitMetric)
	if !approxEqual(v, 7968.75, 1) {
		t.Errorf("fERPM(255) = %f, want ~7968.75", v)
	}
}

func TestFBATT(t *testing.T) {
	// 0.0733 * 0 = 0V
	v, _ := fBATT(0, UnitMetric)
	if v != 0 {
		t.Errorf("fBATT(0) = %f, want 0", v)
	}

	// 0.0733 * 170 ≈ 12.46V (typical battery)
	v, _ = fBATT(170, UnitMetric)
	if !approxEqual(v, 12.461, 0.1) {
		t.Errorf("fBATT(170) = %f, want ~12.46", v)
	}
}

func TestFTIMA(t *testing.T) {
	// x - 10: raw 10 = 0 degrees
	v, s := fTIMA(10, UnitMetric)
	if v != 0 {
		t.Errorf("fTIMA(10) = %f, want 0", v)
	}
	if s != "0°" {
		t.Errorf("fTIMA(10) string = %s, want '0°'", s)
	}

	// raw 0 = -10 degrees
	v, _ = fTIMA(0, UnitMetric)
	if v != -10 {
		t.Errorf("fTIMA(0) = %f, want -10", v)
	}

	// raw 50 = 40 degrees
	v, _ = fTIMA(50, UnitMetric)
	if v != 40 {
		t.Errorf("fTIMA(50) = %f, want 40", v)
	}
}

func TestFTHRL(t *testing.T) {
	// 100 * 0 / 255 = 0%
	v, _ := fTHRL(0, UnitMetric)
	if v != 0 {
		t.Errorf("fTHRL(0) = %f, want 0", v)
	}

	// 100 * 255 / 255 = 100%
	v, _ = fTHRL(255, UnitMetric)
	if !approxEqual(v, 100, 0.1) {
		t.Errorf("fTHRL(255) = %f, want ~100", v)
	}

	// 100 * 128 / 255 ≈ 50.2%
	v, _ = fTHRL(128, UnitMetric)
	if !approxEqual(v, 50.2, 0.5) {
		t.Errorf("fTHRL(128) = %f, want ~50.2", v)
	}
}

func TestFOXYG(t *testing.T) {
	// 0.0195 * 0 = 0V
	v, _ := fOXYG(0, UnitMetric)
	if v != 0 {
		t.Errorf("fOXYG(0) = %f, want 0", v)
	}

	// 0.0195 * 255 ≈ 4.97V
	v, _ = fOXYG(255, UnitMetric)
	if !approxEqual(v, 4.9725, 0.01) {
		t.Errorf("fOXYG(255) = %f, want ~4.97", v)
	}
}

func TestFINJP(t *testing.T) {
	// 0.256 * 0 = 0ms
	v, _ := fINJP(0, UnitMetric)
	if v != 0 {
		t.Errorf("fINJP(0) = %f, want 0", v)
	}

	// 0.256 * 100 = 25.6ms
	v, _ = fINJP(100, UnitMetric)
	if !approxEqual(v, 25.6, 0.1) {
		t.Errorf("fINJP(100) = %f, want 25.6", v)
	}
}

func TestFBARO(t *testing.T) {
	// Metric: 0.00486 * 200 ≈ 0.972 bar
	v, _ := fBARO(200, UnitMetric)
	if !approxEqual(v, 0.972, 0.01) {
		t.Errorf("fBARO(200, metric) = %f, want ~0.972", v)
	}

	// English: 0.972 bar * 14.50326 ≈ 14.1 psi
	v, _ = fBARO(200, UnitEnglish)
	if !approxEqual(v, 14.1, 0.2) {
		t.Errorf("fBARO(200, english) = %f, want ~14.1", v)
	}

	// Raw: just decimal
	v, _ = fBARO(200, UnitRaw)
	if v != 200 {
		t.Errorf("fBARO(200, raw) = %f, want 200", v)
	}
}

func TestFCOOL(t *testing.T) {
	// Test a mid-range value in metric
	v, _ := fCOOL(128, UnitMetric)
	// Should be somewhere around 20-25°C for raw 128
	if v < -50 || v > 200 {
		t.Errorf("fCOOL(128, metric) = %f, out of reasonable range", v)
	}

	// Test in English
	vF, _ := fCOOL(128, UnitEnglish)
	// Fahrenheit should be roughly v*9/5+32
	expectedF := v*9.0/5.0 + 32.0
	if !approxEqual(vF, expectedF, 1.0) {
		t.Errorf("fCOOL(128, english) = %f, expected ~%f", vF, expectedF)
	}
}

func TestFAIRT(t *testing.T) {
	// Test a mid-range value
	v, _ := fAIRT(128, UnitMetric)
	if v < -60 || v > 200 {
		t.Errorf("fAIRT(128, metric) = %f, out of reasonable range", v)
	}

	// English should be C*9/5+32
	vF, _ := fAIRT(128, UnitEnglish)
	expectedF := v*9.0/5.0 + 32.0
	if !approxEqual(vF, expectedF, 1.0) {
		t.Errorf("fAIRT(128, english) = %f, expected ~%f", vF, expectedF)
	}
}

func TestFFTxx(t *testing.T) {
	// 100/128 * 0 = 0%
	v, _ := fFTxx(0, UnitMetric)
	if v != 0 {
		t.Errorf("fFTxx(0) = %f, want 0", v)
	}

	// 100/128 * 128 = 100%
	v, _ = fFTxx(128, UnitMetric)
	if !approxEqual(v, 100, 0.1) {
		t.Errorf("fFTxx(128) = %f, want ~100", v)
	}

	// 100/128 * 255 ≈ 199.2%
	v, _ = fFTxx(255, UnitMetric)
	if !approxEqual(v, 199.2, 0.5) {
		t.Errorf("fFTxx(255) = %f, want ~199.2", v)
	}
}

func TestFAIRF(t *testing.T) {
	// 6.29 * 100 = 629 Hz
	v, _ := fAIRF(100, UnitMetric)
	if !approxEqual(v, 629, 0.5) {
		t.Errorf("fAIRF(100) = %f, want 629", v)
	}
}

func TestFLG2(t *testing.T) {
	// All bits clear: T-A-N (TDC, AC, P/N active), S and I inactive
	// bit 0x04=0 → T, bit 0x08=0 → -, bit 0x10=0 → A, bit 0x20=0 → N, bit 0x80=0 → -
	_, s := fFLG2(0x00, UnitMetric)
	if s != "T-AN-" {
		t.Errorf("fFLG2(0x00) = %s, want 'T-AN-'", s)
	}

	// All relevant bits set
	_, s = fFLG2(0xFF, UnitMetric)
	if s != "-S--I" {
		t.Errorf("fFLG2(0xFF) = %s, want '-S--I'", s)
	}
}
