package money

import "fmt"

// Cents represents money in the smallest unit (no floats).
type Cents int64

func (c Cents) String() string {
	return fmt.Sprintf("%d", int64(c))
}

// RateBPS is interest rate in basis points. 10,000 bps = 100%.
type RateBPS int64

const (
	BPSDenominator RateBPS = 10000
)
