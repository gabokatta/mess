package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mess/internal/domain"
)

const board = `[
  {"casa":"oficial","compra":1485,"venta":1535,"fecha":"2026-09-02"},
  {"casa":"blue","compra":1520,"venta":1540,"fecha":"2026-09-02"},
  {"casa":"bolsa","compra":1532,"venta":1535,"fecha":"2026-09-02"},
  {"casa":"cripto","compra":1550,"venta":1560,"fecha":"2026-09-02"}
]`

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{BaseURL: server.URL, HTTP: server.Client()}
}

func TestOnReturnsTheHousesMessModels(t *testing.T) {
	var path string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(board))
	})

	quotes, err := client.On(context.Background(), time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("On() unexpected error: %v", err)
	}

	if path != "/v1/cotizaciones/dolares/2026/09/02" {
		t.Errorf("requested %q, want the dated cotizaciones path", path)
	}
	if len(quotes) != 3 {
		t.Fatalf("On() returned %d quotes, want 3 (cripto is not a house mess models)", len(quotes))
	}
	want := []Quote{
		{House: domain.Blue, Buy: decimal.NewFromInt(1520), Sell: decimal.NewFromInt(1540)},
		{House: domain.Official, Buy: decimal.NewFromInt(1485), Sell: decimal.NewFromInt(1535)},
		{House: domain.MEP, Buy: decimal.NewFromInt(1532), Sell: decimal.NewFromInt(1535)},
	}
	if diff := cmp.Diff(want, quotes); diff != "" {
		t.Errorf("On() mismatch (-want +got):\n%s", diff)
	}
}

// venta is the field to read: you are the one buying dollars.
func TestSellPicksVenta(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(board))
	})
	quotes, err := client.On(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("On() unexpected error: %v", err)
	}

	sell, ok := Sell(quotes, domain.Blue)
	if !ok {
		t.Fatal("Sell(blue) reported false, want true")
	}
	if !sell.Equal(decimal.NewFromInt(1540)) {
		t.Errorf("Sell(blue) = %s, want 1540", sell)
	}
}

// A completed month's rate is the quote on its last day.
func TestMonthCloseAsksForTheLastDayOfTheMonth(t *testing.T) {
	var path string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(board))
	})

	value, err := client.MonthClose(context.Background(), domain.NewPeriod(2026, time.February), domain.MEP)
	if err != nil {
		t.Fatalf("MonthClose() unexpected error: %v", err)
	}
	if path != "/v1/cotizaciones/dolares/2026/02/28" {
		t.Errorf("requested %q, want february's last day", path)
	}
	if !value.Equal(decimal.NewFromInt(1535)) {
		t.Errorf("MonthClose() = %s, want the bolsa venta 1535", value)
	}
}

func TestOnReportsANonOKStatus(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	if _, err := client.On(context.Background(), time.Now()); err == nil {
		t.Error("On() should report a non-200 response as an error")
	}
}
