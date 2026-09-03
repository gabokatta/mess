package month

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func period(m time.Month) domain.Period { return domain.NewPeriod(2026, m) }

func closeAt(m time.Month, value int64) catalog.FxRate {
	return catalog.FxRate{Period: period(m), Value: decimal.NewFromInt(value), Source: catalog.Close}
}

func manualAt(m time.Month, value int64) catalog.FxRate {
	return catalog.FxRate{Period: period(m), Value: decimal.NewFromInt(value), Source: catalog.Manual}
}

func TestFxTableAt(t *testing.T) {
	september := period(time.September)
	stored := []catalog.FxRate{closeAt(time.July, 1400), closeAt(time.August, 1500)}

	tests := []struct {
		name       string
		stored     []catalog.FxRate
		hasLive    bool
		at         domain.Period
		wantValue  int64
		wantOrigin RateOrigin
		wantFrom   domain.Period
	}{
		{
			name: "the running month is today's quote", stored: stored, hasLive: true,
			at: september, wantValue: 1520, wantOrigin: RateLive,
		},
		{
			name: "a completed month is its own close", stored: stored,
			at: period(time.August), wantValue: 1500, wantOrigin: RateClose,
		},
		{
			name: "a completed month with no close inherits the last one", stored: stored,
			at: period(time.June), wantValue: 0, wantOrigin: RateNone,
		},
		{
			name:   "a gap inherits the close before it",
			stored: []catalog.FxRate{closeAt(time.January, 1100), closeAt(time.April, 1300)},
			at:     period(time.May), wantValue: 1300, wantOrigin: RateInherited, wantFrom: period(time.April),
		},
		{
			name:   "a rate set by hand wins over the live quote",
			stored: append(stored, manualAt(time.September, 1600)), hasLive: true,
			at: september, wantValue: 1600, wantOrigin: RateManual,
		},
		{
			name: "no rate at all", stored: nil,
			at: september, wantOrigin: RateNone,
		},
		{
			name: "no live quote falls back to the last close", stored: stored,
			at: september, wantValue: 1500, wantOrigin: RateInherited, wantFrom: period(time.August),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := NewFxTable(tt.stored, decimal.NewFromInt(1520), tt.hasLive, september)
			got := table.At(tt.at)

			if got.Origin != tt.wantOrigin {
				t.Fatalf("At(%s).Origin = %v, want %v", tt.at, got.Origin, tt.wantOrigin)
			}
			if tt.wantOrigin != RateNone && !got.Value.Equal(decimal.NewFromInt(tt.wantValue)) {
				t.Errorf("At(%s).Value = %s, want %d", tt.at, got.Value, tt.wantValue)
			}
			if tt.wantOrigin == RateInherited && !got.From.Equal(tt.wantFrom) {
				t.Errorf("At(%s).From = %s, want %s", tt.at, got.From, tt.wantFrom)
			}
		})
	}
}

func TestRateLabel(t *testing.T) {
	tests := []struct {
		rate Rate
		want string
	}{
		{Rate{Origin: RateLive}, "live"},
		{Rate{Origin: RateClose}, "close"},
		{Rate{Origin: RateManual}, "manual"},
		{Rate{Origin: RateInherited, From: period(time.August)}, "inherited from 2026-08"},
		{Rate{}, "no rate"},
	}
	for _, tt := range tests {
		if got := tt.rate.Label(); got != tt.want {
			t.Errorf("Label() = %q, want %q", got, tt.want)
		}
	}
}

// Backfill is bounded to the shown year, skips the running month, and never
// refetches a stored close.
func TestMissingCloses(t *testing.T) {
	today := period(time.September)
	stored := []catalog.FxRate{closeAt(time.January, 1100), closeAt(time.March, 1200)}

	got := MissingCloses(2026, today, stored)

	want := []time.Month{time.February, time.April, time.May, time.June, time.July, time.August}
	if len(got) != len(want) {
		t.Fatalf("MissingCloses() = %v, want %v", got, want)
	}
	for i, m := range want {
		if got[i].Month() != m {
			t.Errorf("MissingCloses()[%d] = %s, want %s", i, got[i], m)
		}
	}
}

func TestMissingClosesIsEmptyOnceTheYearIsFilled(t *testing.T) {
	today := period(time.March)
	stored := []catalog.FxRate{closeAt(time.January, 1100), closeAt(time.February, 1150)}

	if got := MissingCloses(2026, today, stored); len(got) != 0 {
		t.Errorf("MissingCloses() = %v, want empty", got)
	}
}
