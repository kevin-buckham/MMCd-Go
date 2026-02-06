package sensor

import (
	"fmt"
)

// UnitSystem controls how temperature and pressure values are displayed.
type UnitSystem int

const (
	UnitMetric  UnitSystem = iota // Celsius, bar
	UnitEnglish                   // Fahrenheit, psi
	UnitRaw                       // raw decimal value
)

// ParseUnitSystem converts a string to a UnitSystem.
func ParseUnitSystem(s string) UnitSystem {
	switch s {
	case "imperial", "english":
		return UnitEnglish
	case "raw", "numeric":
		return UnitRaw
	default:
		return UnitMetric
	}
}

// ConvertFunc takes a raw byte and unit system, returns (float64, formatted string).
type ConvertFunc func(raw byte, units UnitSystem) (float64, string)

// --- Conversion functions ported from format.c ---

// fDEC returns the raw decimal value 0..255.
func fDEC(raw byte, _ UnitSystem) (float64, string) {
	return float64(raw), fmt.Sprintf("%d", raw)
}

// fHEX returns the hexadecimal value.
func fHEX(raw byte, _ UnitSystem) (float64, string) {
	return float64(raw), fmt.Sprintf("%02x", raw)
}

// fFLG0 decodes flags at address 0x00 (AC clutch relay).
func fFLG0(raw byte, _ UnitSystem) (float64, string) {
	s := ""
	if raw&0x20 == 0 {
		s = "A"
	} else {
		s = "-"
	}
	return float64(raw), s
}

// fFLG2 decodes flags at address 0x02 (TDC, P/S, AC, P/N, Idle).
func fFLG2(raw byte, _ UnitSystem) (float64, string) {
	flags := ""
	if raw&0x04 == 0 {
		flags += "T"
	} else {
		flags += "-"
	}
	if raw&0x08 != 0 {
		flags += "S"
	} else {
		flags += "-"
	}
	if raw&0x10 == 0 {
		flags += "A"
	} else {
		flags += "-"
	}
	if raw&0x20 == 0 {
		flags += "N"
	} else {
		flags += "-"
	}
	if raw&0x80 != 0 {
		flags += "I"
	} else {
		flags += "-"
	}
	return float64(raw), flags
}

// Air temperature interpolation table (from format.c)
var airTempInterp = [17]byte{
	0xF4, 0xB0, 0x91, 0x80, 0x74, 0x6A, 0x62, 0x5A,
	0x53, 0x4C, 0x45, 0x3E, 0x35, 0x2B, 0x1D, 0x01,
	0x01,
}

// fAIRT converts air intake temperature using interpolation table, offset -60.
func fAIRT(raw byte, units UnitSystem) (float64, string) {
	if units == UnitRaw {
		return fDEC(raw, units)
	}
	idx := int(raw) / 16
	rem := int(raw) % 16
	v1 := float64(airTempInterp[idx])
	v2 := float64(airTempInterp[idx+1])
	tempC := v1 - float64(rem)*(v1-v2)/16.0 - 60.0

	if units == UnitEnglish {
		tempF := tempC*9.0/5.0 + 32.0
		return tempF, fmt.Sprintf("%.1f\u00b0F", tempF)
	}
	return tempC, fmt.Sprintf("%.1f\u00b0C", tempC)
}

// Coolant temperature interpolation table (from format.c)
var coolantTempInterp = [17]byte{
	0xEE, 0xBE, 0xA0, 0x90, 0x84, 0x7B, 0x73, 0x6C,
	0x65, 0x5F, 0x58, 0x51, 0x49, 0x40, 0x33, 0x15,
	0x15,
}

// fCOOL converts coolant temperature using interpolation table, offset -80.
func fCOOL(raw byte, units UnitSystem) (float64, string) {
	if units == UnitRaw {
		return fDEC(raw, units)
	}
	idx := int(raw) / 16
	rem := int(raw) % 16
	v1 := float64(coolantTempInterp[idx])
	v2 := float64(coolantTempInterp[idx+1])
	tempC := v1 - float64(rem)*(v1-v2)/16.0 - 80.0

	if units == UnitEnglish {
		tempF := tempC*9.0/5.0 + 32.0
		return tempF, fmt.Sprintf("%.1f\u00b0F", tempF)
	}
	return tempC, fmt.Sprintf("%.1f\u00b0C", tempC)
}

// fEGRT converts EGR temperature: -1.5x + 314.27 (Celsius).
func fEGRT(raw byte, units UnitSystem) (float64, string) {
	if units == UnitRaw {
		return fDEC(raw, units)
	}
	tempC := -1.5*float64(raw) + 314.27

	if units == UnitEnglish {
		tempF := tempC*9.0/5.0 + 32.0
		return tempF, fmt.Sprintf("%.1f\u00b0F", tempF)
	}
	return tempC, fmt.Sprintf("%.1f\u00b0C", tempC)
}

// fBATT converts battery voltage: 0.0733x volts.
func fBATT(raw byte, _ UnitSystem) (float64, string) {
	v := 0.0733 * float64(raw)
	return v, fmt.Sprintf("%.1fV", v)
}

// fERPM converts engine speed: 31.25x rpm.
func fERPM(raw byte, _ UnitSystem) (float64, string) {
	v := 31.25 * float64(raw)
	return v, fmt.Sprintf("%.0frpm", v)
}

// fINJP converts injector pulse width: 0.256x ms.
func fINJP(raw byte, _ UnitSystem) (float64, string) {
	v := 0.256 * float64(raw)
	return v, fmt.Sprintf("%.2fms", v)
}

// fBARO converts barometric pressure: 0.00486x bar.
func fBARO(raw byte, units UnitSystem) (float64, string) {
	if units == UnitRaw {
		return fDEC(raw, units)
	}
	bar := 0.00486 * float64(raw)
	if units == UnitEnglish {
		psi := bar * 14.50326
		return psi, fmt.Sprintf("%.2fpsi", psi)
	}
	return bar, fmt.Sprintf("%.3fbar", bar)
}

// fAIRF converts mass air flow sensor frequency: 6.29x Hz.
func fAIRF(raw byte, _ UnitSystem) (float64, string) {
	v := 6.29 * float64(raw)
	return v, fmt.Sprintf("%.1fHz", v)
}

// fTHRL converts throttle position / percentage: 100x/255 %.
func fTHRL(raw byte, _ UnitSystem) (float64, string) {
	v := 100.0 * float64(raw) / 255.0
	return v, fmt.Sprintf("%.1f%%", v)
}

// fFTxx converts fuel trim values: 0.78x % (range 0..200%).
func fFTxx(raw byte, _ UnitSystem) (float64, string) {
	v := 100.0 * float64(raw) / 128.0 // 0.78125 * x, matching old code's 51200/65536
	return v, fmt.Sprintf("%.1f%%", v)
}

// fOXYG converts oxygen sensor voltage: 0.0195x V (range 0..~5V).
func fOXYG(raw byte, _ UnitSystem) (float64, string) {
	v := 0.0195 * float64(raw)
	return v, fmt.Sprintf("%.3fV", v)
}

// fTIMA converts timing advance: x - 10 degrees.
func fTIMA(raw byte, _ UnitSystem) (float64, string) {
	v := float64(raw) - 10.0
	return v, fmt.Sprintf("%.0f\u00b0", v)
}
