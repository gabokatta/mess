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

	asked := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	quotes, err := client.On(context.Background(), asked)
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
		{House: domain.Blue, Buy: decimal.NewFromInt(1520), Sell: decimal.NewFromInt(1540), Date: asked},
		{House: domain.Official, Buy: decimal.NewFromInt(1485), Sell: decimal.NewFromInt(1535), Date: asked},
		{House: domain.MEP, Buy: decimal.NewFromInt(1532), Sell: decimal.NewFromInt(1535), Date: asked},
	}
	if diff := cmp.Diff(want, quotes); diff != "" {
		t.Errorf("On() mismatch (-want +got):\n%s", diff)
	}
}

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

func TestOnWalksBackToTheLastTradingDay(t *testing.T) {
	var asked []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path != "/v1/cotizaciones/dolares/2026/09/04" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(board))
	})

	saturday := time.Date(2026, time.September, 5, 0, 0, 0, 0, time.UTC)
	quotes, err := client.On(context.Background(), saturday)
	if err != nil {
		t.Fatalf("On() unexpected error: %v", err)
	}
	if len(quotes) != 3 {
		t.Fatalf("On() returned %d quotes, want 3", len(quotes))
	}

	friday := saturday.AddDate(0, 0, -1)
	if !quotes[0].Date.Equal(friday) {
		t.Errorf("quote is dated %s, want the Friday it was actually quoted on", quotes[0].Date)
	}
	want := []string{"/v1/cotizaciones/dolares/2026/09/05", "/v1/cotizaciones/dolares/2026/09/04"}
	if diff := cmp.Diff(want, asked); diff != "" {
		t.Errorf("requests (-want +got):\n%s", diff)
	}
}

func TestOnGivesUpAfterAWeekOfClosedDays(t *testing.T) {
	var calls int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := client.On(context.Background(), time.Now()); err == nil {
		t.Fatal("On() returned no error after a week with no quotes")
	}
	if calls != lookback+1 {
		t.Errorf("made %d requests, want %d — the day asked for plus a week behind it", calls, lookback+1)
	}
}

func TestOnDoesNotWalkBackPastARealFailure(t *testing.T) {
	var calls int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := client.On(context.Background(), time.Now()); err == nil {
		t.Fatal("On() swallowed a 500")
	}
	if calls != 1 {
		t.Errorf("made %d requests, want 1 — an outage is not a closed market", calls)
	}
}

func TestMonthCloseReadsTheLastTradingDayOfTheMonth(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		// February 2026 ends on Saturday the 28th.
		if r.URL.Path != "/v1/cotizaciones/dolares/2026/02/27" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(board))
	})

	close, err := client.MonthClose(context.Background(),
		domain.NewPeriod(2026, time.February), domain.Blue)
	if err != nil {
		t.Fatalf("MonthClose() unexpected error: %v", err)
	}
	if !close.Equal(decimal.NewFromInt(1540)) {
		t.Errorf("MonthClose() = %s, want Friday's blue venta 1540", close)
	}
}
