// Package dolarapi fetches Argentine dollar quotes from dolarapi.com.
package dolarapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/domain"
)

// houseSlug is each FxHouse's path segment on dolarapi.com. MEP trades as
// "dólar bolsa" there, hence "bolsa" rather than "mep".
var houseSlug = map[domain.FxHouse]string{
	domain.Blue:     "blue",
	domain.Official: "oficial",
	domain.MEP:      "bolsa",
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{BaseURL: "https://dolarapi.com", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

type quoteResponse struct {
	Venta decimal.Decimal `json:"venta"`
}

// Quote fetches house's current sell rate — venta, what you pay to buy
// dollars — the rate relevant to money leaving pesos.
func (c *Client) Quote(ctx context.Context, house domain.FxHouse) (decimal.Decimal, error) {
	slug, ok := houseSlug[house]
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("dolarapi: unknown fx house %s", house)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/dolares/"+slug, nil)
	if err != nil {
		return decimal.Decimal{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return decimal.Decimal{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decimal.Decimal{}, fmt.Errorf("dolarapi: unexpected status %d", resp.StatusCode)
	}

	var q quoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		return decimal.Decimal{}, err
	}
	return q.Venta, nil
}
