// Package rates fetches Argentine dollar quotes from api.argentinadatos.com,
// which answers for any date back to 2011 and carries closes over weekends.
package rates

import (
	"context"
	"encoding/json"
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
// conversion runs on.
type Quote struct {
	House domain.FxHouse
	Buy   decimal.Decimal
	Sell  decimal.Decimal
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

// On returns the houses in Houses order, dropping any unmodelled house the
// API reports.
func (c *Client) On(ctx context.Context, date time.Time) ([]Quote, error) {
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
			quotes = append(quotes, Quote{House: house, Buy: r.Buy, Sell: r.Sell})
		}
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

// MonthClose reads the last day of period; a completed month's rate is
// fetched once and never again.
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
