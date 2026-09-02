package tui

import (
	"database/sql"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gabokatta/mess/internal/catalog"
	"github.com/gabokatta/mess/internal/dolarapi"
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/month"
)

type view int

const (
	viewMonth view = iota
	viewYear
	viewConcepts
	viewLists
	viewSettings
)

var viewNames = [...]string{"Month", "Year", "Concepts", "Lists", "Settings"}

func (v view) String() string { return viewNames[v] }

type Model struct {
	theme             Theme
	view              view
	width             int
	height            int
	db                *sql.DB
	fxClient          *dolarapi.Client
	period            domain.Period
	lines             []month.Line
	loadErr           error
	cursor            int
	editing           *editState
	saveErr           error
	fxErr             error
	year              month.Year
	yearErr           error
	yearSeries        trendSeries
	yearConceptCursor int
	yearDrillDown     *catalog.Concept

	allocations       []catalog.SavingAllocation
	rates             []catalog.FxRate
	allocationsErr    error
	allocationForm    *allocationFormState
	allocationSaveErr error

	incomeConfirmForm  *incomeConfirmFormState
	incomeConfirmShown map[domain.Period]bool

	lastMonthUnfinished int
	lastMonthChoresErr  error

	lists            []catalog.List
	listsErr         error
	listCursor       int
	showClosed       bool
	listEditing      *listEditState
	listSaveErr      error
	newList          *newListFormState
	periodAssignForm *periodAssignFormState

	concepts        []catalog.Concept
	categories      []catalog.Category
	baseAmounts     map[int64][]catalog.BaseAmount
	conceptsErr     error
	conceptCursor   int
	conceptForm     *conceptFormState
	conceptEditForm *conceptEditFormState
	conceptSaveErr  error

	settings        catalog.Settings
	settingsErr     error
	settingsForm    *settingsFormState
	settingsSaveErr error

	dbPath     string
	exportForm *exportFormState
	importForm *importFormState
	backupMsg  string
	backupErr  error

	fxOverrideForm *fxOverrideFormState
	fxOverrideErr  error
}

func New(db *sql.DB) Model {
	return Model{
		theme:              NewTheme(true),
		db:                 db,
		fxClient:           dolarapi.NewClient(),
		period:             domain.PeriodFromTime(time.Now()),
		incomeConfirmShown: make(map[domain.Period]bool),
	}
}

// WithDBPath attaches the on-disk database path, which the Settings view's
// export/import actions need for backup.Snapshot but the rest of the app
// never touches. Optional: tests that don't exercise backup can skip it.
func (m Model) WithDBPath(path string) Model {
	m.dbPath = path
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		loadMonth(m.db, m.period),
		loadAllocations(m.db, m.period),
		loadLastMonthChores(m.db, m.period),
		fillCurrentFxRate(m.db, m.fxClient, m.period),
		loadYear(m.db, m.period.Year()),
		loadLists(m.db),
		ensureDefaultCategories(m.db),
		loadConcepts(m.db),
		loadSettings(m.db),
	)
}

// loadView reloads whatever view v shows, so a write made in one view is
// never stale in another.
func (m Model) loadView(v view) tea.Cmd {
	switch v {
	case viewMonth:
		return tea.Batch(loadMonth(m.db, m.period), loadAllocations(m.db, m.period), loadLastMonthChores(m.db, m.period))
	case viewYear:
		return loadYear(m.db, m.period.Year())
	case viewLists:
		return loadLists(m.db)
	case viewConcepts:
		return loadConcepts(m.db)
	case viewSettings:
		return loadSettings(m.db)
	default:
		return nil
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = NewTheme(msg.IsDark())

	case monthLoadedMsg:
		m.lines, m.loadErr = msg.lines, msg.err
		if !m.incomeConfirmShown[m.period] && msg.err == nil {
			if form := m.maybeIncomeConfirmForm(); form != nil {
				m.incomeConfirmShown[m.period] = true
				m.incomeConfirmForm = form
				return m, form.form.Init()
			}
		}

	case entrySavedMsg:
		m.saveErr = msg.err
		return m, loadMonth(m.db, m.period)

	case allocationsLoadedMsg:
		m.allocations, m.rates, m.allocationsErr = msg.allocations, msg.rates, msg.err

	case allocationSavedMsg:
		m.allocationSaveErr = msg.err
		return m, loadAllocations(m.db, m.period)

	case lastMonthChoresLoadedMsg:
		m.lastMonthUnfinished, m.lastMonthChoresErr = msg.unfinished, msg.err

	case incomeConfirmedMsg:
		return m, loadMonth(m.db, m.period)

	case backupDoneMsg:
		m.backupMsg, m.backupErr = msg.message, msg.err
		return m, loadSettings(m.db)

	case fxOverrideMsg:
		m.fxOverrideErr = msg.err

	case fxFilledMsg:
		m.fxErr = msg.err

	case yearLoadedMsg:
		m.year, m.yearErr = msg.year, msg.err

	case listsLoadedMsg:
		m.lists, m.listsErr = msg.lists, msg.err

	case listSavedMsg:
		m.listSaveErr = msg.err
		return m, loadLists(m.db)

	case categoriesSeededMsg:
		return m, loadConcepts(m.db)

	case conceptsLoadedMsg:
		m.concepts, m.categories, m.baseAmounts, m.conceptsErr = msg.concepts, msg.categories, msg.baseAmounts, msg.err

	case conceptSavedMsg:
		m.conceptSaveErr = msg.err
		return m, tea.Batch(loadConcepts(m.db), loadMonth(m.db, m.period))

	case settingsLoadedMsg:
		m.settings, m.settingsErr = msg.settings, msg.err

	case settingsSavedMsg:
		m.settingsSaveErr = msg.err
		return m, loadSettings(m.db)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	default:
		// Non-key messages from an open Huh form (field/group advancement)
		// round-trip back through here rather than handleKey.
		if m.conceptForm != nil {
			return m.forwardConceptForm(msg)
		}
		if m.settingsForm != nil {
			return m.forwardSettingsForm(msg)
		}
		if m.newList != nil {
			return m.forwardNewList(msg)
		}
		if m.allocationForm != nil {
			return m.forwardAllocationForm(msg)
		}
		if m.incomeConfirmForm != nil {
			return m.forwardIncomeConfirmForm(msg)
		}
		if m.exportForm != nil {
			return m.forwardExportForm(msg)
		}
		if m.importForm != nil {
			return m.forwardImportForm(msg)
		}
		if m.conceptEditForm != nil {
			return m.forwardConceptEditForm(msg)
		}
		if m.fxOverrideForm != nil {
			return m.forwardFxOverrideForm(msg)
		}
		if m.periodAssignForm != nil {
			return m.forwardPeriodAssignForm(msg)
		}
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.editing != nil {
		return m.updateEditing(msg)
	}
	if m.listEditing != nil {
		return m.updateListEditing(msg)
	}
	if m.newList != nil {
		return m.updateNewList(msg)
	}
	if m.conceptForm != nil {
		return m.updateConceptForm(msg)
	}
	if m.settingsForm != nil {
		return m.updateSettingsForm(msg)
	}
	if m.allocationForm != nil {
		return m.updateAllocationForm(msg)
	}
	if m.incomeConfirmForm != nil {
		return m.updateIncomeConfirmForm(msg)
	}
	if m.exportForm != nil {
		return m.updateExportForm(msg)
	}
	if m.importForm != nil {
		return m.updateImportForm(msg)
	}
	if m.conceptEditForm != nil {
		return m.updateConceptEditForm(msg)
	}
	if m.fxOverrideForm != nil {
		return m.updateFxOverrideForm(msg)
	}
	if m.periodAssignForm != nil {
		return m.updatePeriodAssignForm(msg)
	}
	if m.yearDrillDown != nil {
		return m.updateYearDrillDown(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab", "l":
		m.view = (m.view + 1) % view(len(viewNames))
		return m, m.loadView(m.view)
	case "shift+tab", "h":
		m.view = (m.view - 1 + view(len(viewNames))) % view(len(viewNames))
		return m, m.loadView(m.view)
	case "j", "down":
		if m.view == viewMonth {
			m.cursor = m.moveCursor(1)
		} else if m.view == viewLists {
			m.listCursor = m.moveListCursor(1)
		} else if m.view == viewConcepts {
			m.conceptCursor = m.moveConceptCursor(1)
		} else if m.view == viewYear {
			m.yearConceptCursor = m.moveYearConceptCursor(1)
		}
	case "k", "up":
		if m.view == viewMonth {
			m.cursor = m.moveCursor(-1)
		} else if m.view == viewLists {
			m.listCursor = m.moveListCursor(-1)
		} else if m.view == viewConcepts {
			m.conceptCursor = m.moveConceptCursor(-1)
		} else if m.view == viewYear {
			m.yearConceptCursor = m.moveYearConceptCursor(-1)
		}
	case "[":
		if m.view == viewMonth {
			return m.shiftPeriod(-1)
		}
	case "]":
		if m.view == viewMonth {
			return m.shiftPeriod(1)
		}
	case "space":
		if m.view == viewMonth {
			return m.toggleDone()
		} else if m.view == viewLists {
			return m.toggleListCheckbox()
		}
	case "e":
		if m.view == viewMonth {
			return m.startEdit()
		} else if m.view == viewLists {
			return m.startListEdit()
		} else if m.view == viewSettings {
			return m.startSettingsEdit()
		} else if m.view == viewConcepts {
			return m.startConceptEdit()
		}
	case "c":
		if m.view == viewLists {
			return m.toggleListClosed()
		}
	case "p":
		if m.view == viewLists {
			return m.startPeriodAssign()
		}
	case "f":
		if m.view == viewLists {
			m.showClosed = !m.showClosed
			m.listCursor = 0
		}
	case "n":
		if m.view == viewLists {
			return m.startNewList()
		} else if m.view == viewConcepts {
			return m.startNewConcept()
		}
	case "a":
		if m.view == viewMonth {
			return m.startAllocationPanel()
		}
	case "d":
		if m.view == viewMonth {
			return m.deleteCursorAllocation()
		}
	case "s":
		if m.view == viewYear {
			m.yearSeries = nextTrendSeries(m.yearSeries)
		}
	case "enter":
		if m.view == viewYear {
			return m.startYearDrillDown()
		}
	case "x":
		if m.view == viewSettings {
			return m.startExport()
		}
	case "i":
		if m.view == viewSettings {
			return m.startImport()
		}
	case "r":
		if m.view == viewSettings {
			return m.startFxOverride()
		}
	}
	return m, nil
}

// Below this floor renderTooSmall takes over instead of a garbled layout.
const (
	minUsableWidth  = 40
	minUsableHeight = 10
)

func (m Model) View() tea.View {
	content := m.renderTooSmall()
	if content == "" {
		content = m.renderApp()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	v.WindowTitle = "mess"
	return v
}

// centerInBox centers content inside the app's box, both horizontally and
// vertically — the same treatment renderTooSmall already gives the "grow
// your terminal" message, applied here to any view whose content is
// shorter than the box: an empty state, a lone drill-down chart, Settings'
// handful of fields.
func (m Model) centerInBox(content string) string {
	w := m.width - 6
	h := m.height - 6
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

// renderTooSmall reports the "grow your terminal" message once a real,
// too-small size is known, or "" when the normal layout should render.
func (m Model) renderTooSmall() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width >= minUsableWidth && m.height >= minUsableHeight {
		return ""
	}
	msg := m.theme.Muted.Width(m.width).Align(lipgloss.Center).Render("make the terminal bigger to see your mess")
	return lipgloss.PlaceVertical(m.height, lipgloss.Center, msg)
}

func (m Model) renderApp() string {
	footer := m.renderFooter()
	footerRows := strings.Count(footer, "\n") + 1
	boxHeight := m.height - footerRows
	app := m.theme.App
	if m.width > 0 && boxHeight > 0 {
		app = app.Width(m.width).Height(boxHeight)
	}
	rendered := app.Render(m.viewContent())
	if m.width >= logoMinWidth && m.height >= logoMinHeight {
		rendered = overlayLogo(rendered, m.theme.Logo)
	}

	return rendered + "\n" + footer
}

// renderFooter is the strip below the box: just the tab strip, right-aligned
// — the key legend lives inside the box now, as the last line of each
// view's own content.
func (m Model) renderFooter() string {
	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(m.tabs())
}

func (m Model) tabs() string {
	labels := make([]string, len(viewNames))
	for i, name := range viewNames {
		style := m.theme.Tab
		if view(i) == m.view {
			style = m.theme.TabActive
		}
		labels[i] = style.Render(name)
	}
	return strings.Join(labels, "")
}

// viewContent renders the focused view's body plus its own help line, the
// last line of every view's content — form or not — now that the footer
// carries only the tab strip.
func (m Model) viewContent() string {
	var body string
	switch m.view {
	case viewMonth:
		body = m.renderMonth()
	case viewYear:
		body = m.renderYear()
	case viewLists:
		body = m.renderLists()
	case viewConcepts:
		body = m.renderConcepts()
	case viewSettings:
		body = m.renderSettings()
	default:
		body = m.theme.Muted.Render(m.view.String() + " — not built yet")
	}
	return body + "\n\n" + m.theme.Help.Render(m.helpText())
}

func (m Model) helpText() string {
	if m.editing != nil {
		return "enter confirm · esc cancel"
	}
	if m.listEditing != nil {
		return "ctrl+s save · esc cancel"
	}
	if m.newList != nil || m.conceptForm != nil || m.settingsForm != nil || m.allocationForm != nil ||
		m.incomeConfirmForm != nil || m.exportForm != nil || m.importForm != nil ||
		m.conceptEditForm != nil || m.fxOverrideForm != nil || m.periodAssignForm != nil {
		return "esc cancel"
	}
	if m.view == viewMonth {
		return "↑/↓ move · space tick · e edit · a allocate · d delete allocation · [/] month · tab/shift+tab switch · q quit"
	}
	if m.view == viewLists {
		return "↑/↓ move · space tick · e edit · c close · p period · f pending/closed · n new · tab/shift+tab switch · q quit"
	}
	if m.view == viewConcepts {
		return "↑/↓ move · e edit · n new · tab/shift+tab switch · q quit"
	}
	if m.view == viewSettings {
		return "e edit · x export · i import · r fx rate · tab/shift+tab switch · q quit"
	}
	if m.view == viewYear && m.yearDrillDown != nil {
		return "esc back"
	}
	if m.view == viewYear {
		return "↑/↓ move · enter drill down · s cycle trend series · tab/shift+tab switch · q quit"
	}
	return "tab/shift+tab switch · q quit"
}
