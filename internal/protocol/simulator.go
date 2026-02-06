package protocol

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// Simulator generates fake ECU sensor data for UI development and testing.
// It cycles through driving scenarios: idle → rev → cruise → decel → idle.
type Simulator struct {
	mu      sync.Mutex
	defs    []sensor.Definition
	running bool
	tick    float64 // simulation time in seconds
	rng     *rand.Rand
}

// NewSimulator creates a new ECU data simulator.
func NewSimulator(defs []sensor.Definition) *Simulator {
	return &Simulator{
		defs: defs,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// PollSensors generates a simulated sample for the given sensor indices.
func (s *Simulator) PollSensors(indices []int) (sensor.Sample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tick += 0.05 // ~20Hz simulation rate

	var sample sensor.Sample
	sample.Time = time.Now()

	// Driving cycle: 60-second loop
	// 0-10s: idle
	// 10-20s: acceleration (revving up)
	// 20-40s: cruise
	// 40-50s: deceleration
	// 50-60s: idle
	cyclePos := math.Mod(s.tick, 60.0)

	var rpmTarget, tpsTarget, coolTarget float64
	var timingTarget, injpTarget float64

	switch {
	case cyclePos < 10: // idle
		rpmTarget = 850
		tpsTarget = 0
		coolTarget = 82
		timingTarget = 10
		injpTarget = 3.0
	case cyclePos < 20: // acceleration
		progress := (cyclePos - 10) / 10.0
		rpmTarget = 850 + progress*5150 // up to 6000
		tpsTarget = 30 + progress*60    // up to 90%
		coolTarget = 82 + progress*8    // warming up
		timingTarget = 10 + progress*25
		injpTarget = 3.0 + progress*15.0
	case cyclePos < 40: // cruise
		rpmTarget = 3200
		tpsTarget = 25
		coolTarget = 90
		timingTarget = 32
		injpTarget = 8.0
	case cyclePos < 50: // deceleration
		progress := (cyclePos - 40) / 10.0
		rpmTarget = 3200 - progress*2350 // down to 850
		tpsTarget = 25 - progress*25     // closing
		coolTarget = 90 - progress*8
		timingTarget = 32 - progress*22
		injpTarget = 8.0 - progress*5.0
	default: // idle again
		rpmTarget = 850
		tpsTarget = 0
		coolTarget = 82
		timingTarget = 10
		injpTarget = 3.0
	}

	// Add noise
	noise := func(base, amplitude float64) float64 {
		return base + (s.rng.Float64()-0.5)*2*amplitude
	}

	for _, idx := range indices {
		if idx < 0 || idx >= len(s.defs) {
			continue
		}
		def := s.defs[idx]
		if !def.Exists || def.Computed {
			continue
		}

		var raw byte
		switch def.Slug {
		case "RPM":
			raw = byte(clamp(noise(rpmTarget, 30)/31.25, 0, 255))
		case "TPS":
			raw = byte(clamp(noise(tpsTarget, 1)*255/100, 0, 255))
		case "COOL":
			// Reverse the coolant temp interpolation to get raw value
			// ~82°C maps to roughly raw 100, ~90°C to raw 85
			raw = byte(clamp(noise(200-coolTarget*1.2, 2), 0, 255))
		case "TIMA":
			raw = byte(clamp(noise(timingTarget+10, 1), 0, 255))
		case "KNCK":
			// Occasional knock during high RPM
			if rpmTarget > 4000 && s.rng.Float64() < 0.15 {
				raw = byte(s.rng.Intn(5) + 1)
			} else {
				raw = 0
			}
		case "INJP":
			raw = byte(clamp(noise(injpTarget/0.256, 0.5), 0, 255))
		case "BATT":
			// ~14.2V while running
			raw = byte(clamp(noise(14.2/0.0733, 0.5), 0, 255))
		case "O2-R", "O2-F":
			// Oscillate between lean/rich (0.2-0.8V)
			o2v := 0.45 + 0.35*math.Sin(s.tick*3.0+float64(idx))
			raw = byte(clamp(o2v/0.0195, 0, 255))
		case "BARO":
			// ~1.01 bar (sea level)
			raw = byte(clamp(noise(1.01/0.00486, 0.3), 0, 255))
		case "ISC":
			// Higher at idle, lower when driving
			if rpmTarget < 1000 {
				raw = byte(clamp(noise(35, 2)*255/100, 0, 255))
			} else {
				raw = byte(clamp(noise(10, 1)*255/100, 0, 255))
			}
		case "MAFS":
			raw = byte(clamp(noise(rpmTarget*0.08/6.29, 1), 0, 255))
		case "AIRT":
			// ~25°C intake air
			raw = byte(clamp(noise(128, 2), 0, 255))
		case "EGRT":
			// Hotter under load
			raw = byte(clamp(noise((314.27-150+tpsTarget*0.5)/1.5, 2), 0, 255))
		case "FTRL", "FTRM", "FTRH":
			// Hover around 100% (stoich)
			raw = byte(clamp(noise(128, 3), 0, 255))
		case "FTO2":
			raw = byte(clamp(noise(128, 5), 0, 255))
		case "ACLE":
			// Spikes during throttle changes
			if tpsTarget > 50 {
				raw = byte(clamp(noise(tpsTarget*0.5*255/100, 3), 0, 255))
			} else {
				raw = byte(clamp(noise(5, 2), 0, 255))
			}
		case "FLG0":
			raw = 0x20 // AC off
		case "FLG2":
			if rpmTarget < 1000 {
				raw = 0x80 // idle flag set
			} else {
				raw = 0x00
			}
		default:
			raw = byte(clamp(noise(128, 10), 0, 255))
		}

		sample.SetData(idx, raw)
	}

	// Compute INJD
	sample.ComputeDerivatives(s.defs)

	return sample, nil
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
