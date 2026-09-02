package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
)

func TestResolvePeriodStatus(t *testing.T) {
	today := domain.NewPeriod(2026, time.September)

	tests := []struct {
		name string
		p    domain.Period
		want periodStatus
	}{
		{"before today", domain.NewPeriod(2026, time.August), periodPast},
		{"same month as today", today, periodCurrent},
		{"after today", domain.NewPeriod(2026, time.October), periodFuture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePeriodStatus(tt.p, today); got != tt.want {
				t.Errorf("resolvePeriodStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBracketKeysShiftTheMonthPeriodAndReload(t *testing.T) {
	db := openTestStore(t)
	period := domain.NewPeriod(2026, time.September)
	m := New(db)
	m.width, m.height = 100, 40
	m.period = period
	m.financeCursor = 1
	m.choreCursor = 1

	updated, cmd := m.Update(key("["))
	m = updated.(Model)
	if !m.period.Equal(domain.NewPeriod(2026, time.August)) {
		t.Errorf("period = %v, want August after [", m.period)
	}
	if m.financeCursor != 0 || m.choreCursor != 0 {
		t.Errorf("financeCursor = %d, choreCursor = %d, want both reset to 0 after a period shift", m.financeCursor, m.choreCursor)
	}
	if cmd == nil {
		t.Fatal("[ returned no Cmd, want a reload")
	}

	updated, cmd = m.Update(key("]"))
	m = updated.(Model)
	if !m.period.Equal(period) {
		t.Errorf("period = %v, want back to September after ]", m.period)
	}
	_ = cmd
}

func TestMonthViewRendersPeriodStatus(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	m.period = domain.PeriodFromTime(time.Now()).AddMonths(-2)

	updated, _ := m.Update(monthLoadedMsg{})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "past") {
		t.Errorf("content = %q, want the period marked past", content)
	}
}

func TestIncomeConfirmPromptsAgainForEachDistinctPeriod(t *testing.T) {
	db := openTestStore(t)
	sept := domain.NewPeriod(2026, time.September)
	seedConceptOfKind(t, db, "Sueldo", catalog.Income, 1000000)

	m := monthModel(t, db, sept)
	if m.incomeConfirmForm == nil {
		t.Fatal("incomeConfirmForm = nil, want it opened for September")
	}
	updated, _ := m.Update(keyEsc())
	m = updated.(Model)

	updated, cmd := m.Update(key("["))
	m = updated.(Model)
	m = settle(t, m, cmd)
	if m.incomeConfirmForm == nil {
		t.Fatal("incomeConfirmForm = nil, want it to reopen for the newly shown August (never seen this session)")
	}
}
