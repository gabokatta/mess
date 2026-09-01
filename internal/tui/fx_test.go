package tui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/dolarapi"
	"github.com/gabokatta/mes/internal/domain"
)

func TestFillCurrentFxRateWritesQuoteWhenPeriodEmpty(t *testing.T) {
	db := openTestStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"venta": 1215}`))
	}))
	defer server.Close()
	client := &dolarapi.Client{BaseURL: server.URL, HTTP: server.Client()}
	period := domain.NewPeriod(2026, time.September)

	msg := fillCurrentFxRate(db, client, period)()

	m, ok := msg.(fxFilledMsg)
	if !ok || m.err != nil {
		t.Fatalf("fillCurrentFxRate() = %#v, want fxFilledMsg with no error", msg)
	}
	rates, err := catalog.FxRates(db)
	if err != nil {
		t.Fatalf("FxRates() unexpected error: %v", err)
	}
	if len(rates) != 1 || !rates[0].Period.Equal(period) || rates[0].Source != catalog.Fetched {
		t.Fatalf("FxRates() = %+v, want a single Fetched row for %s", rates, period)
	}
}

func TestFillCurrentFxRateSurfacesFetchError(t *testing.T) {
	db := openTestStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := &dolarapi.Client{BaseURL: server.URL, HTTP: server.Client()}

	msg := fillCurrentFxRate(db, client, domain.NewPeriod(2026, time.September))()

	m, ok := msg.(fxFilledMsg)
	if !ok || m.err == nil {
		t.Fatalf("fillCurrentFxRate() = %#v, want fxFilledMsg with an error", msg)
	}
}

func TestUpdateSurfacesFxFetchError(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	updated, _ := m.Update(fxFilledMsg{err: errors.New("boom")})
	m = updated.(Model)

	content := m.View().Content
	if !strings.Contains(content, "fx quote unavailable") {
		t.Errorf("month view content = %q, want it to surface the fx fetch error", content)
	}
}
