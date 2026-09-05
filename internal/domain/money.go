package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

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

func ParseCurrency(s string) (Currency, error) {
	switch s {
	case "ARS":
		return ARS, nil
	case "USD":
		return USD, nil
	default:
		return 0, fmt.Errorf("domain: invalid currency %q", s)
	}
}

// Money keeps currency attached to amounts until conversion.
type Money struct {
	amount   decimal.Decimal
	currency Currency
}

func NewMoney(amount decimal.Decimal, currency Currency) Money {
	return Money{amount: amount, currency: currency}
}

func (m Money) Amount() decimal.Decimal { return m.amount }
func (m Money) Currency() Currency      { return m.currency }

func (m Money) Equal(other Money) bool {
	return m.currency == other.currency && m.amount.Equal(other.amount)
}

// ToARS converts m at rate pesos per dollar. Reports false for a USD amount
// with no rate, so callers drop the line rather than count it as zero.
func (m Money) ToARS(rate decimal.Decimal, hasRate bool) (Money, bool) {
	if m.currency == ARS {
		return m, true
	}
	if !hasRate {
		return Money{}, false
	}
	return Money{amount: m.amount.Mul(rate), currency: ARS}, true
}

func (m Money) String() string {
	return m.currency.String() + " " + m.amount.StringFixed(2)
}
