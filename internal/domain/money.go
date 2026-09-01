package domain

import "fmt"

type Currency int

const (
	ARS Currency = iota
	USD
)

func (c Currency) String() string {
	switch c {
	case ARS:
		return "ARS"
	case USD:
		return "USD"
	default:
		return fmt.Sprintf("Currency(%d)", int(c))
	}
}

type Money struct {
	amount   int64
	currency Currency
}

func NewMoney(amount int64, currency Currency) Money {
	return Money{amount: amount, currency: currency}
}

func (m Money) Amount() int64      { return m.amount }
func (m Money) Currency() Currency { return m.currency }

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("domain: cannot add %s to %s", other.currency, m.currency)
	}
	return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("domain: cannot subtract %s from %s", other.currency, m.currency)
	}
	return Money{amount: m.amount - other.amount, currency: m.currency}, nil
}

func (m Money) Negate() Money {
	return Money{amount: -m.amount, currency: m.currency}
}

func (m Money) Share(bp int64) Money {
	return Money{amount: shareHalfUp(m.amount, bp), currency: m.currency}
}

const basisPointsDenominator = 10000

// Go's integer division truncates toward zero, so half-up on a negative
// numerator has to round the positive magnitude and negate back, not just
// add half the denominator before dividing.
func shareHalfUp(amount, bp int64) int64 {
	numerator := amount * bp
	if numerator >= 0 {
		return (numerator + basisPointsDenominator/2) / basisPointsDenominator
	}
	return -((-numerator + basisPointsDenominator/2) / basisPointsDenominator)
}
