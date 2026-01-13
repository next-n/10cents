package domain

import (
	"testing"

	"gateway/internal/money"
)

func TestInterestPolicy_FloorRules(t *testing.T) {
	p := DefaultPolicy()

	t.Run("attempt=0 amount=10 => 100% => interest=10", func(t *testing.T) {
		got := p.InterestDue(money.Cents(10), 0)
		if got != money.Cents(10) {
			t.Fatalf("interest = %d, want %d", got, 10)
		}
	})

	t.Run("attempt=10 amount=10 => 110% => interest=11", func(t *testing.T) {
		got := p.InterestDue(money.Cents(10), 10)
		if got != money.Cents(11) {
			t.Fatalf("interest = %d, want %d", got, 11)
		}
	})

	t.Run("attempt=10 amount=1 => 110% => floor(1.1)=1", func(t *testing.T) {
		got := p.InterestDue(money.Cents(1), 10)
		if got != money.Cents(1) {
			t.Fatalf("interest = %d, want %d", got, 1)
		}
	})

	t.Run("attempt=1 amount=10 => 101% => floor(10.1)=10", func(t *testing.T) {
		got := p.InterestDue(money.Cents(10), 1)
		if got != money.Cents(10) {
			t.Fatalf("interest = %d, want %d", got, 10)
		}
	})

	t.Run("attempt=100 amount=10 => 200% => interest=20", func(t *testing.T) {
		got := p.InterestDue(money.Cents(10), 100)
		if got != money.Cents(20) {
			t.Fatalf("interest = %d, want %d", got, 20)
		}
	})
}
