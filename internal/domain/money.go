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

const currencyScale = 2

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

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("domain: cannot add %s to %s", other.currency, m.currency)
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("domain: cannot subtract %s from %s", other.currency, m.currency)
	}
	return Money{amount: m.amount.Sub(other.amount), currency: m.currency}, nil
}

func (m Money) Negate() Money {
	return Money{amount: m.amount.Neg(), currency: m.currency}
}

func (m Money) Share(p Percent) Money {
	return Money{amount: m.amount.Mul(p.Fraction()).Round(currencyScale), currency: m.currency}
}
