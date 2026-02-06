package sensor

import "time"

// Sample represents a single snapshot of all polled sensor values.
type Sample struct {
	Time        time.Time        `json:"time"`
	DataPresent uint32           `json:"dataPresent"` // bitmask of which sensors have data
	RawData     [MaxSensors]byte `json:"rawData"`     // raw byte values from ECU
}

// HasData returns true if the sensor at the given index has data in this sample.
func (s *Sample) HasData(idx int) bool {
	return s.DataPresent&(1<<uint(idx)) != 0
}

// SetData sets the raw value for a sensor index and marks it present.
func (s *Sample) SetData(idx int, value byte) {
	s.RawData[idx] = value
	s.DataPresent |= 1 << uint(idx)
}

// ConvertedValues returns a map of slug -> formatted string for all present sensors.
func (s *Sample) ConvertedValues(defs []Definition, units UnitSystem) map[string]string {
	result := make(map[string]string, len(defs))
	for i, def := range defs {
		if !def.Exists || !s.HasData(i) {
			continue
		}
		result[def.Slug] = def.Format(s.RawData[i], units)
	}
	return result
}

// ConvertedFloats returns a map of slug -> float64 for all present sensors.
func (s *Sample) ConvertedFloats(defs []Definition, units UnitSystem) map[string]float64 {
	result := make(map[string]float64, len(defs))
	for i, def := range defs {
		if !def.Exists || !s.HasData(i) {
			continue
		}
		result[def.Slug] = def.Convert(s.RawData[i], units)
	}
	return result
}

// ComputeDerivatives calculates derived values like injector duty cycle.
// Must be called after all raw sensor data is collected for this sample.
func (s *Sample) ComputeDerivatives(defs []Definition) {
	rpmIdx := -1
	injpIdx := -1
	injdIdx := -1

	for i, def := range defs {
		switch def.Slug {
		case "RPM":
			rpmIdx = i
		case "INJP":
			injpIdx = i
		case "INJD":
			injdIdx = i
		}
	}

	if rpmIdx < 0 || injpIdx < 0 || injdIdx < 0 {
		return
	}

	if s.HasData(rpmIdx) && s.HasData(injpIdx) {
		// Injector duty cycle = (IPW_raw * RPM_raw) / 117, capped at 255
		v := int32(s.RawData[injpIdx]) * int32(s.RawData[rpmIdx]) / 117
		if v > 255 {
			v = 255
		}
		s.SetData(injdIdx, byte(v))
	}
}
