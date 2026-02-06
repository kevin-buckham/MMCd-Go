package sensor

import "fmt"

// MaxSensors is the maximum number of sensor slots (matches original SENSOR_COUNT).
const MaxSensors = 32

// Definition describes a single ECU sensor: its address, name, and how to convert raw data.
type Definition struct {
	Addr        byte        `json:"addr"`        // ECU address byte
	Slug        string      `json:"slug"`        // Short name (4 chars, e.g. "RPM")
	Description string      `json:"description"` // Human-readable description
	Unit        string      `json:"unit"`        // Display unit
	Exists      bool        `json:"exists"`      // Whether this sensor slot is active
	Computed    bool        `json:"computed"`    // True if derived (e.g. INJD), not directly polled
	convertFunc ConvertFunc // conversion function
}

// Format returns a human-readable string for the raw value.
func (d *Definition) Format(raw byte, units UnitSystem) string {
	if d.convertFunc == nil {
		_, s := fDEC(raw, units)
		return s
	}
	_, s := d.convertFunc(raw, units)
	return s
}

// Convert returns a float64 for the raw value.
func (d *Definition) Convert(raw byte, units UnitSystem) float64 {
	if d.convertFunc == nil {
		v, _ := fDEC(raw, units)
		return v
	}
	v, _ := d.convertFunc(raw, units)
	return v
}

// DefaultDefinitions returns the full sensor table matching the original mmcd panel.c.
// Indices match the original code for compatibility with the binary log format.
func DefaultDefinitions() []Definition {
	defs := make([]Definition, MaxSensors)

	// Index 0: unused placeholder
	defs[0] = Definition{Addr: 0xFF, Slug: "", Description: "", Exists: false, convertFunc: fDEC}

	// Index 1: Flags 0 (AC clutch)
	defs[1] = Definition{Addr: 0x00, Slug: "FLG0", Description: "Flags 0 (AC clutch)", Unit: "flags", Exists: true, convertFunc: fFLG0}

	// Index 2: Flags 2 (TDC, P/S, AC, P/N, Idle)
	defs[2] = Definition{Addr: 0x02, Slug: "FLG2", Description: "Flags 2 (TDC/PS/AC/PN/Idle)", Unit: "flags", Exists: true, convertFunc: fFLG2}

	// Index 3: Timing Advance
	defs[3] = Definition{Addr: 0x06, Slug: "TIMA", Description: "Timing advance", Unit: "deg", Exists: true, convertFunc: fTIMA}

	// Index 4: Coolant Temperature
	defs[4] = Definition{Addr: 0x07, Slug: "COOL", Description: "Coolant temp", Unit: "deg", Exists: true, convertFunc: fCOOL}

	// Index 5: Fuel Trim Low
	defs[5] = Definition{Addr: 0x0C, Slug: "FTRL", Description: "Fuel trim low", Unit: "%", Exists: true, convertFunc: fFTxx}

	// Index 6: Fuel Trim Middle
	defs[6] = Definition{Addr: 0x0D, Slug: "FTRM", Description: "Fuel trim middle", Unit: "%", Exists: true, convertFunc: fFTxx}

	// Index 7: Fuel Trim High
	defs[7] = Definition{Addr: 0x0E, Slug: "FTRH", Description: "Fuel trim high", Unit: "%", Exists: true, convertFunc: fFTxx}

	// Index 8: O2 Feedback Trim
	defs[8] = Definition{Addr: 0x0F, Slug: "FTO2", Description: "O2 feedback trim", Unit: "%", Exists: true, convertFunc: fFTxx}

	// Index 9: EGR Temperature
	defs[9] = Definition{Addr: 0x12, Slug: "EGRT", Description: "EGR temp", Unit: "deg", Exists: true, convertFunc: fEGRT}

	// Index 10: O2 Sensor (rear)
	defs[10] = Definition{Addr: 0x13, Slug: "O2-R", Description: "O2 sensor (rear)", Unit: "V", Exists: true, convertFunc: fOXYG}

	// Index 11: Battery Voltage
	defs[11] = Definition{Addr: 0x14, Slug: "BATT", Description: "Battery", Unit: "V", Exists: true, convertFunc: fBATT}

	// Index 12: Barometric Pressure
	defs[12] = Definition{Addr: 0x15, Slug: "BARO", Description: "Barometer", Unit: "bar", Exists: true, convertFunc: fBARO}

	// Index 13: ISC Steps
	defs[13] = Definition{Addr: 0x16, Slug: "ISC", Description: "ISC position", Unit: "%", Exists: true, convertFunc: fTHRL}

	// Index 14: Throttle Position
	defs[14] = Definition{Addr: 0x17, Slug: "TPS", Description: "Throttle position", Unit: "%", Exists: true, convertFunc: fTHRL}

	// Index 15: Mass Air Flow Frequency
	defs[15] = Definition{Addr: 0x1A, Slug: "MAFS", Description: "Mass air flow", Unit: "Hz", Exists: true, convertFunc: fAIRF}

	// Index 16: Acceleration Enrichment
	defs[16] = Definition{Addr: 0x1D, Slug: "ACLE", Description: "Accel enrichment", Unit: "%", Exists: true, convertFunc: fTHRL}

	// Index 17: Engine Speed (RPM)
	defs[17] = Definition{Addr: 0x21, Slug: "RPM", Description: "Engine speed", Unit: "rpm", Exists: true, convertFunc: fERPM}

	// Index 18: Knock Sum
	defs[18] = Definition{Addr: 0x26, Slug: "KNCK", Description: "Knock sum", Unit: "count", Exists: true, convertFunc: fDEC}

	// Index 19: Injector Pulse Width
	defs[19] = Definition{Addr: 0x29, Slug: "INJP", Description: "Inj pulse width", Unit: "ms", Exists: true, convertFunc: fINJP}

	// Index 20: Injector Duty Cycle (computed from RPM + INJP)
	defs[20] = Definition{Addr: 0xFF, Slug: "INJD", Description: "Inj duty cycle", Unit: "%", Exists: true, Computed: true, convertFunc: fFTxx}

	// Index 21: Air Intake Temperature
	defs[21] = Definition{Addr: 0x3A, Slug: "AIRT", Description: "Air temp", Unit: "deg", Exists: true, convertFunc: fAIRT}

	// Index 22: O2 Sensor (front)
	defs[22] = Definition{Addr: 0x3E, Slug: "O2-F", Description: "O2 sensor (front)", Unit: "V", Exists: true, convertFunc: fOXYG}

	// Indices 23-31: unused / custom sensor slots
	for i := 23; i < MaxSensors; i++ {
		slug := ""
		if i >= 24 {
			slug = fmt.Sprintf("%02X", 0)
		}
		defs[i] = Definition{Addr: 0x00, Slug: slug, Description: "", Exists: false, convertFunc: fDEC}
	}

	return defs
}

// ActiveDefinitions returns only the definitions that exist and are not computed.
// These are the sensors that can be polled from the ECU.
func ActiveDefinitions(defs []Definition) []Definition {
	var active []Definition
	for _, d := range defs {
		if d.Exists && !d.Computed {
			active = append(active, d)
		}
	}
	return active
}

// FindBySlug returns the index and definition for a given slug, or -1 if not found.
func FindBySlug(defs []Definition, slug string) (int, *Definition) {
	for i := range defs {
		if defs[i].Slug == slug {
			return i, &defs[i]
		}
	}
	return -1, nil
}

// FindByAddr returns the index and definition for a given ECU address, or -1 if not found.
func FindByAddr(defs []Definition, addr byte) (int, *Definition) {
	for i := range defs {
		if defs[i].Exists && defs[i].Addr == addr {
			return i, &defs[i]
		}
	}
	return -1, nil
}

// SlugsToIndices converts a list of sensor slugs to their indices.
// Returns an error string for any slugs not found.
func SlugsToIndices(defs []Definition, slugs []string) ([]int, []string) {
	var indices []int
	var notFound []string
	for _, slug := range slugs {
		idx, _ := FindBySlug(defs, slug)
		if idx >= 0 {
			indices = append(indices, idx)
		} else {
			notFound = append(notFound, slug)
		}
	}
	return indices, notFound
}

// AllPollableIndices returns indices of all sensors that can be polled (exist, not computed, addr != 0xFF).
func AllPollableIndices(defs []Definition) []int {
	var indices []int
	for i, d := range defs {
		if d.Exists && !d.Computed && d.Addr != 0xFF {
			indices = append(indices, i)
		}
	}
	return indices
}

// CommonSensorSlugs returns the default "common" subset for quick logging.
var CommonSensorSlugs = []string{
	"RPM", "TPS", "COOL", "TIMA", "KNCK", "INJP", "O2-R", "BATT",
}
