package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/rates"
)

func houseIndex(house domain.FxHouse) int {
	for i, h := range rates.Houses {
		if h == house {
			return i
		}
	}
	return 0
}

func (m Model) handleRatesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		house := rates.Houses[clamp(m.house, len(rates.Houses))]
		return m, write(func() error { return catalog.SetFxHouse(m.db, house) })
	case "e":
		return m.openModal(m.manualRateForm())
	}
	return m, nil
}

func (m Model) manualRateForm() *form {
	period := m.period
	value := m.fx().At(period).Value.StringFixed(2)

	return newForm(m.theme, m.width, m.height,
		[]*huh.Group{
			huh.NewGroup(
				huh.NewInput().Title("Rate for " + period.String()).
					Description("pesos per dollar").
					Value(&value).Validate(validateDecimal),
			).Title("Set rate by hand"),
		},
		func() tea.Cmd {
			return write(func() error {
				parsed, err := decimal.NewFromString(value)
				if err != nil {
					return err
				}
				return catalog.SetManualFxRate(m.db, period, parsed)
			})
		})
}

func (m Model) renderRates() string {
	rate := m.fx().At(m.period)

	left := m.theme.Muted.Render("Rates · ") + m.theme.Title.Render(m.period.String())
	right := m.theme.Muted.Render(rate.Label() + " · " + strings.ToLower(m.settings.FxHouse.String()))

	var b strings.Builder
	b.WriteString(m.spread(left, right))
	b.WriteString("\n\n")
	b.WriteString(m.renderHouses())
	b.WriteString("\n\n")
	b.WriteString(groupStyle(1).Render("MONTHLY CLOSE"))
	b.WriteString("\n")
	b.WriteString(m.renderCloseChart())
	b.WriteString("\n")
	b.WriteString(m.renderRateSummary(rate))
	return b.String()
}

func (m Model) renderHouses() string {
	if len(m.quotes) == 0 {
		return m.theme.Muted.Render("no quotes today — the app runs on stored closes until one lands")
	}

	rows := make([]string, len(m.quotes))
	for i, q := range m.quotes {
		cursor := "  "
		if i == m.house {
			cursor = m.theme.Accent.Render("> ")
		}
		name := m.theme.Bright.Width(14).Render(q.House.String())
		pair := m.theme.Muted.Width(20).Render(formatAmount(q.Buy) + " / " + formatAmount(q.Sell))

		using := ""
		if q.House == m.settings.FxHouse {
			using = m.theme.Accent.Render("using")
		}
		rows[i] = cursor + name + pair + using
	}
	return strings.Join(rows, "\n")
}

func (m Model) yearCloses() []decimal.Decimal {
	fx := m.fx()
	closes := make([]decimal.Decimal, 12)
	for i := range closes {
		p := domain.NewPeriod(m.period.Year(), time.Month(i+1))
		if rate := fx.At(p); rate.Origin != month.RateInherited {
			closes[i] = rate.Value
		}
	}
	return closes
}

func (m Model) renderCloseChart() string {
	closes := m.yearCloses()
	bars := make([]barchart.BarData, len(closes))
	for i, value := range closes {
		style := m.theme.Muted
		if i+1 == int(m.period.Month()) {
			style = m.theme.Accent
		}
		bars[i] = barchart.BarData{
			Label:  shortMonth(time.Month(i + 1)),
			Values: []barchart.BarValue{{Name: "close", Value: value.InexactFloat64(), Style: style}},
		}
	}
	return drawBars(bars, chartWidth(m.width), min(max(m.bodyHeight(2)-9, 4), 10))
}

func (m Model) renderRateSummary(rate month.Rate) string {
	left := m.theme.Muted.Render(strings.ToLower(m.period.Month().String()) + ": ")
	if !rate.OK() {
		return left + m.theme.Muted.Render("no rate yet")
	}
	left += formatAmount(rate.Value) + " " + m.theme.Muted.Render(rate.Label())

	january := m.yearCloses()[0]
	if january.IsZero() {
		return left
	}
	move := rate.Value.Sub(january).Div(january).Mul(decimal.NewFromInt(100))
	return m.spread(left, m.theme.Muted.Render(signed(move)+"% since january"))
}

func signed(d decimal.Decimal) string {
	if d.IsNegative() {
		return formatAmount(d)
	}
	return "+" + formatAmount(d)
}

func shortMonth(m time.Month) string { return strings.ToLower(m.String()[:3]) }
