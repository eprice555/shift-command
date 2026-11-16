package engine

// FixedStep converts a fixed frame-update rate into a slower fixed sim rate
// using an integer accumulator, so the sim never sees floating-point drift.
type FixedStep struct {
	simTPS, updateTPS int
	acc               int
}

// NewFixedStep returns a stepper running simTPS sim ticks per updateTPS updates.
func NewFixedStep(simTPS, updateTPS int) *FixedStep {
	if simTPS <= 0 || updateTPS <= 0 {
		panic("engine: tick rates must be positive")
	}
	return &FixedStep{simTPS: simTPS, updateTPS: updateTPS}
}

// Advance records one update and returns how many sim ticks to run now.
func (f *FixedStep) Advance() int {
	f.acc += f.simTPS
	n := 0
	for f.acc >= f.updateTPS {
		f.acc -= f.updateTPS
		n++
	}
	return n
}

// Alpha is the fraction [0,1) of the way to the next sim tick, for
// interpolating rendering between ticks.
func (f *FixedStep) Alpha() float64 {
	return float64(f.acc) / float64(f.updateTPS)
}
