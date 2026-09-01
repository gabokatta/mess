package domain

import "github.com/shopspring/decimal"

type Percent struct {
	fraction decimal.Decimal
}

var percentDenominator = decimal.NewFromInt(100)

func NewPercent(wholePercent int64) Percent {
	return Percent{fraction: decimal.NewFromInt(wholePercent).Div(percentDenominator)}
}

func NewPercentFromFraction(fraction decimal.Decimal) Percent {
	return Percent{fraction: fraction}
}

func (p Percent) Fraction() decimal.Decimal { return p.fraction }
