// Package rates fetches Argentine dollar quotes from api.argentinadatos.com,
// which answers for any date the market traded on, back to 2011. It has no
// route for the latest quote and 404s on a day nobody traded, so a date the
// caller asks for is the newest date it will accept, not the one it answers
// with.
package rates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// MEP trades as "bolsa" in this API.
var houseSlug = map[domain.FxHouse]string{
	domain.Blue:     "blue",
	domain.Official: "oficial",
	domain.MEP:      "bolsa",
}

// Quote's Sell is venta, what you pay to buy dollars, and so the rate every
// conversion runs on. Date is the day it was quoted, which is not always the
// day it was asked for: a Saturday's quotes are Friday's.
type Quote struct {
	House domain.FxHouse
	Buy   decimal.Decimal
	Sell  decimal.Decimal
	Date  time.Time
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: "https://api.argentinadatos.com",
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

type quoteRow struct {
	House string          `json:"casa"`
	Buy   decimal.Decimal `json:"compra"`
	Sell  decimal.Decimal `json:"venta"`
}

// errClosed is a day the market did not trade on. It is the one failure worth
// retrying against an earlier date; everything else is a real outage and is
// reported as it happened.
var errClosed = errors.New("rates: no quotes for that day")

// lookback is how far On walks back for a day the market was open. Argentina
// can put a holiday beside a weekend and close for four days, and a week is
// enough to clear the longest of those without papering over a real outage.
const lookback = 7

// On returns the houses in Houses order for date, or for the most recent
// trading day before it. Weekends and holidays have no quotes at all, so
// asking for today on a Saturday would otherwise leave the app with none.
func (c *Client) On(ctx context.Context, date time.Time) ([]Quote, error) {
	for back := 0; back <= lookback; back++ {
		quotes, err := c.on(ctx, date.AddDate(0, 0, -back))
		if err == nil {
			return quotes, nil
		}
		if !errors.Is(err, errClosed) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("rates: no quotes in the %d days to %s", lookback, date.Format(time.DateOnly))
}

// on returns the houses in Houses order, dropping any unmodelled house the
// API reports.
func (c *Client) on(ctx context.Context, date time.Time) ([]Quote, error) {
	url := fmt.Sprintf("%s/v1/cotizaciones/dolares/%04d/%02d/%02d",
		c.BaseURL, date.Year(), int(date.Month()), date.Day())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errClosed
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rates: %s returned %d", url, resp.StatusCode)
	}

	var rows []quoteRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	bySlug := make(map[string]quoteRow, len(rows))
	for _, r := range rows {
		bySlug[r.House] = r
	}
	var quotes []Quote
	for _, house := range Houses {
		if r, ok := bySlug[houseSlug[house]]; ok {
			quotes = append(quotes, Quote{House: house, Buy: r.Buy, Sell: r.Sell, Date: date})
		}
	}
	if len(quotes) == 0 {
		return nil, errClosed
	}
	return quotes, nil
}

// Houses is the display order of every house mess converts with.
var Houses = []domain.FxHouse{domain.Blue, domain.Official, domain.MEP}

func Sell(quotes []Quote, house domain.FxHouse) (decimal.Decimal, bool) {
	for _, q := range quotes {
		if q.House == house {
			return q.Sell, true
		}
	}
	return decimal.Decimal{}, false
}

// MonthClose reads the last day of period, or the last day it traded on: a
// month ending on a weekend has no quote on its final date, and roughly two
// months in seven end that way.
func (c *Client) MonthClose(ctx context.Context, period domain.Period, house domain.FxHouse) (decimal.Decimal, error) {
	lastDay := time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	quotes, err := c.On(ctx, lastDay)
	if err != nil {
		return decimal.Decimal{}, err
	}
	sell, ok := Sell(quotes, house)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("rates: no %s quote for %s", house, period)
	}
	return sell, nil
}
