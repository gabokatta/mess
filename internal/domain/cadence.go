package domain

import "time"

type Cadence uint16

// Monthly sets bits 0-11, one per calendar month.
const Monthly Cadence = 1<<12 - 1

func NewCadence(months ...time.Month) Cadence {
	var c Cadence
	for _, m := range months {
		c |= cadenceBit(m)
	}
	return c
}

// time.Month is 1-indexed (January == 1), so shift by m-1 to land on bit 0.
func cadenceBit(m time.Month) Cadence {
	return 1 << (m - 1)
}

func (c Cadence) Occurs(p Period) bool {
	return c&cadenceBit(p.Month()) != 0
}

var Aguinaldo = NewCadence(time.June, time.December)
