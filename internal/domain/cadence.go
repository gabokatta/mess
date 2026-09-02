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

// Months decodes c back into the months it sets, January first — the
// inverse of NewCadence, for prefilling a months picker from a stored value.
func (c Cadence) Months() []time.Month {
	var months []time.Month
	for m := time.January; m <= time.December; m++ {
		if c&cadenceBit(m) != 0 {
			months = append(months, m)
		}
	}
	return months
}

var Aguinaldo = NewCadence(time.June, time.December)
