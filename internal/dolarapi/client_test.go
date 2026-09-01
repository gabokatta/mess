package dolarapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

// blueResponse is a real dolarapi.com response body for /v1/dolares/blue.
const blueResponse = `{
	"moneda": "USD",
	"casa": "blue",
	"nombre": "Blue",
	"compra": 1195,
	"venta": 1215,
	"fechaActualizacion": "2026-09-01T14:30:00.000Z"
}`

func TestQuoteReturnsVenta(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(blueResponse))
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	got, err := c.Quote(context.Background(), domain.Blue)
	if err != nil {
		t.Fatalf("Quote() unexpected error: %v", err)
	}
	if !got.Equal(decimal.NewFromInt(1215)) {
		t.Errorf("Quote() = %s, want 1215 (venta)", got)
	}
	if gotPath != "/v1/dolares/blue" {
		t.Errorf("request path = %q, want /v1/dolares/blue", gotPath)
	}
}

func TestQuoteMapsHouseToSlug(t *testing.T) {
	tests := []struct {
		house domain.FxHouse
		slug  string
	}{
		{domain.Blue, "blue"},
		{domain.Official, "oficial"},
		{domain.MEP, "bolsa"},
	}
	for _, tt := range tests {
		var gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(blueResponse))
		}))

		c := &Client{BaseURL: server.URL, HTTP: server.Client()}
		if _, err := c.Quote(context.Background(), tt.house); err != nil {
			t.Fatalf("Quote(%s) unexpected error: %v", tt.house, err)
		}
		if want := "/v1/dolares/" + tt.slug; gotPath != want {
			t.Errorf("Quote(%s) hit %q, want %q", tt.house, gotPath, want)
		}
		server.Close()
	}
}

func TestQuoteReturnsErrorOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	if _, err := c.Quote(context.Background(), domain.Blue); err == nil {
		t.Error("Quote() error = nil, want an error on a 500 response")
	}
}
