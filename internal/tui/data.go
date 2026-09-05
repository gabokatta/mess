package tui

import (
	"context"
	"database/sql"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
	"github.com/gabokatta/mess/internal/rates"
)

type monthMsg struct {
	lines []month.Line
	seq   uint64
	err   error
}

type yearMsg struct {
	year month.Year
	seq  uint64
	err  error
}

type notesMsg struct {
	notes []catalog.Note
	err   error
}

type catalogMsg struct {
	concepts   []catalog.Concept
	categories []catalog.Category
	err        error
}

type ratesMsg struct {
	stored   []catalog.FxRate
	settings catalog.Settings
	err      error
}

type quotesMsg struct {
	quotes []rates.Quote
	err    error
}

type savedMsg struct {
	err error
}

type backfilledMsg struct {
	saved int
}

func (m *Model) loadMonth() tea.Cmd {
	m.monthSeq++
	db, period, seq := m.db, m.period, m.monthSeq
	return func() tea.Msg {
		loaded, err := month.Load(db, period)
		return monthMsg{lines: loaded.Lines, seq: seq, err: err}
	}
}

func (m *Model) loadYear() tea.Cmd {
	m.yearSeq++
	db, year, fx, seq := m.db, m.period.Year(), m.fx(), m.yearSeq
	return func() tea.Msg {
		y, err := month.LoadYear(db, year, fx)
		return yearMsg{year: y, seq: seq, err: err}
	}
}

func loadNotes(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		notes, err := catalog.Notes(db)
		return notesMsg{notes: notes, err: err}
	}
}

func loadCatalog(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		concepts, err := catalog.Concepts(db)
		if err != nil {
			return catalogMsg{err: err}
		}
		categories, err := catalog.Categories(db)
		return catalogMsg{concepts: concepts, categories: categories, err: err}
	}
}

func loadRates(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		stored, err := catalog.FxRates(db)
		if err != nil {
			return ratesMsg{err: err}
		}
		settings, err := catalog.LoadSettings(db)
		return ratesMsg{stored: stored, settings: settings, err: err}
	}
}

func seedCategories(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		return savedMsg{err: catalog.EnsureDefaultCategories(db)}
	}
}

func fetchQuotes(client *rates.Client) tea.Cmd {
	return func() tea.Msg {
		quotes, err := client.On(context.Background(), time.Now())
		return quotesMsg{quotes: quotes, err: err}
	}
}

func backfillCloses(db *sql.DB, client *rates.Client, year int, today domain.Period) tea.Cmd {
	return func() tea.Msg {
		house, err := catalog.FxHouse(db)
		if err != nil {
			return backfilledMsg{}
		}
		stored, err := catalog.FxRates(db)
		if err != nil {
			return backfilledMsg{}
		}

		saved := 0
		for _, p := range month.MissingCloses(year, today, stored) {
			value, err := client.MonthClose(context.Background(), p, house)
			if err != nil {
				continue
			}
			if catalog.SaveFxClose(db, p, value, house) == nil {
				saved++
			}
		}
		return backfilledMsg{saved: saved}
	}
}

// Refetch after clearing, then reload even if no replacement quote was available.
func clearRate(db *sql.DB, client *rates.Client, period, today domain.Period) tea.Cmd {
	return func() tea.Msg {
		if err := catalog.ClearFxRate(db, period); err != nil {
			return savedMsg{err: err}
		}
		backfillCloses(db, client, period.Year(), today)()
		return savedMsg{}
	}
}

func write(fn func() error) tea.Cmd {
	return func() tea.Msg { return savedMsg{err: fn()} }
}
