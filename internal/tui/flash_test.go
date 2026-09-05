package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/fixture"
)

// The reload that follows a write reports success on five messages, and every
// one of them used to overwrite the refusal that started it, so a refused
// action looked exactly like an ignored one.
func TestARefusalSurvivesTheReloadThatFollowsIt(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)
	m.view = viewConcepts
	m = m.sync()

	m, _ = send(t, m, savedMsg{err: errors.New("Home holds 2 concepts")})
	if m.flash == "" {
		t.Fatal("a refused write left nothing on screen")
	}

	m, _ = send(t, m,
		runCmd(t, loadMonth(m.db, m.period)),
		runCmd(t, loadCatalog(m.db)),
		runCmd(t, loadNotes(m.db)),
		runCmd(t, loadRates(m.db)),
		runCmd(t, loadYear(m.db, m.period.Year(), m.fx())),
	)
	if m.flash != "Home holds 2 concepts" {
		t.Errorf("flash = %q after the reload, want the refusal still there", m.flash)
	}
}

// Alert, and in the status line the app already keeps for it.
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

// An older message's timer must not take a newer one off the screen.
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

// A refusal you have since worked around should not still be on screen.
func TestASuccessfulWriteTakesTheRefusalOffTheScreen(t *testing.T) {
	m := modelFor(t, catalogWorld(), minUsableWidth, 32)

	m, _ = send(t, m, savedMsg{err: errors.New("Home holds 2 concepts")})
	m, _ = send(t, m, savedMsg{})
	if m.flash != "" {
		t.Errorf("flash = %q, want a successful write to clear it", m.flash)
	}
}

// End to end, through the key that was doing nothing visible: the confirm is
// answered, the delete is refused, and the screen says why.
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

// A load that fails is worth the same line, so an unreadable catalog is not a
// silently empty one.
func TestALoadFailureFlashesToo(t *testing.T) {
	m := modelFor(t, fixture.World{}, minUsableWidth, 32)

	m, _ = send(t, m, catalogMsg{err: errors.New("catalog: database is locked")})
	if m.flash != "catalog: database is locked" {
		t.Errorf("flash = %q, want the load error", m.flash)
	}
}
