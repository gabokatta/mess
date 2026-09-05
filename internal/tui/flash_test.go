package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

func TestARefusalSurvivesTheReloadThatFollowsIt(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	m, _ = send(t, m, savedMsg{err: errors.New("Home holds 2 concepts")})
	if m.flash == "" {
		t.Fatal("a refused write left nothing on screen")
	}

	monthLoad := m.loadMonth()
	m, _ = send(t, m,
		runCmd(t, monthLoad),
		runCmd(t, loadCatalog(m.db)),
		runCmd(t, loadNotes(m.db)),
		runCmd(t, loadRates(m.db)),
	)
	yearLoad := m.loadYear()
	m, _ = send(t, m, runCmd(t, yearLoad))
	if m.flash != "Home holds 2 concepts" {
		t.Errorf("flash = %q after the reload, want the refusal still there", m.flash)
	}
}

func TestARefusalRendersInAlert(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m, _ = send(t, m, savedMsg{err: errors.New("Home holds 2 concepts")})

	content := m.View().Content
	if !strings.Contains(content, m.theme.Alert.Bold(true).Render("Home holds 2 concepts")) {
		t.Errorf("the refusal is not drawn in Alert:\n%s", stripANSI(content))
	}
}

func TestAFlashClearsWhenItsTimerFires(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m, _ = send(t, m, savedMsg{err: errors.New("Home holds 2 concepts")})

	m, _ = send(t, m, flashExpired{seq: m.flashSeq})
	if m.flash != "" {
		t.Errorf("flash = %q, want the timer to have cleared it", m.flash)
	}
}

func TestAnOldTimerDoesNotClearANewerFlash(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	m, _ = send(t, m, savedMsg{err: errors.New("first")})
	stale := m.flashSeq
	m, _ = send(t, m, savedMsg{err: errors.New("second")})

	m, _ = send(t, m, flashExpired{seq: stale})
	if m.flash != "second" {
		t.Errorf("flash = %q, want the newer message to survive the older timer", m.flash)
	}
}

func TestASuccessfulWriteTakesTheRefusalOffTheScreen(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	m, _ = send(t, m, savedMsg{err: errors.New("Home holds 2 concepts")})
	m, _ = send(t, m, savedMsg{})
	if m.flash != "" {
		t.Errorf("flash = %q, want a successful write to clear it", m.flash)
	}
}

func TestDeletingACategoryHoldingConceptsSaysWhy(t *testing.T) {
	m, list := openCategories(t, catalogWorld())
	var home catalog.Category
	for _, c := range list.categories {
		if c.Name == "Home" {
			home = c
		}
	}

	saved := savedMsg{err: catalog.DeleteCategory(m.db, home.ID)}
	if saved.err == nil {
		t.Fatal("deleting a category holding concepts should be refused")
	}

	m, _ = send(t, m, saved)
	if !strings.Contains(m.View().Content, "Home holds 2 concepts") {
		t.Errorf("the screen does not say why the delete did nothing:\n%s", stripANSI(m.View().Content))
	}
}

func TestALoadFailureFlashesToo(t *testing.T) {
	m := modelFor(t, fixture.World{}, minUsableWidth, 32)

	m, _ = send(t, m, catalogMsg{err: errors.New("catalog: database is locked")})
	if m.flash != "catalog: database is locked" {
		t.Errorf("flash = %q, want the load error", m.flash)
	}
}
