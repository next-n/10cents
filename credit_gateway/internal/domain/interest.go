package domain

import (
	"gateway/internal/money"
)

// InterestPolicy defines the stupid rules.
type InterestPolicy struct {
	// BaseRateBPS: start at 100% => 10,000 bps
	BaseRateBPS money.RateBPS
	// StepBPSPerAttempt: +0.1% per attempt => 10 bps
	StepBPSPerAttempt money.RateBPS
	// InvalidAmountPenaltyMultiplier: invalid amount fee = multiplier * current interest
	// InvalidAmountPenaltyMultiplier int64
}

const InvalidAmountFineCents = 1000 // $10.00

func DefaultPolicy() InterestPolicy {
	return InterestPolicy{
		BaseRateBPS:       10000,
		StepBPSPerAttempt: 100, //1%
		// InvalidAmountPenaltyMultiplier: 10,
	}
}

// RateBPS = 100% + (attempt_count * 0.1%)
func (p InterestPolicy) RateBPS(attemptCount int64) money.RateBPS {
	if attemptCount < 0 {
		attemptCount = 0
	}

	return p.BaseRateBPS + money.RateBPS(attemptCount)*p.StepBPSPerAttempt
}

// InterestDue = floor(spent_cents * rate_bps / 10000)
func (p InterestPolicy) InterestDue(spent money.Cents, attemptCount int64) money.Cents {

	rate := p.RateBPS(attemptCount)
	return money.Cents(int64(spent) * (int64(rate)) / int64(money.BPSDenominator))
}

// InvalidAmountFee = multiplier * InterestDue
// func (p InterestPolicy) InvalidAmountFee(spent money.Cents, attemptCount int64) money.Cents {
// 	interest := p.InterestDue(spent, attemptCount)
// 	return money.Cents(int64(interest) * p.InvalidAmountPenaltyMultiplier)
// }
