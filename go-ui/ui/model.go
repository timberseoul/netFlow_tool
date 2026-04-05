package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"netFlow_tool-ui/service"
	"netFlow_tool-ui/types"
)

// tickMsg triggers a UI refresh at a fixed interval.
// The tick is purely for rendering — data is fetched independently
// by the StatsService background goroutine.
type tickMsg time.Time

const (
	modeRealtime  = "realtime"
	modeHistory   = "history"
	sortMenuWidth = 24

	menuNone   = ""
	menuSort   = "sort"
	menuFilter = "filter"
	menuExit   = "exit"

	historySortDate  = "date"
	historySortTotal = "total"
)

var sortMenuItems = []struct {
	key   string
	label string
}{
	{"download", "Download"},
	{"upload", "Upload"},
	{"name", "Name"},
	{"pid", "PID"},
	{"order", "Order"},
}

var filterMenuItems = []struct {
	key   string
	label string
}{
	{"", "All"},
	{"user", "User"},
	{"system", "System"},
	{"service", "Service"},
}

// Model is the bubbletea model for the TUI.
type Model struct {
	statsSvc   *service.StatsService
	webPort    string
	flows      []types.ProcessFlow
	history    []types.DailyUsage
	err        error
	historyErr error

	mode          string
	width         int
	height        int
	sortBy        string
	sortAsc       bool
	historySortBy string
	quitting      bool

	filterCategory string
	pageIndex      int

	activeMenu string
	menuIndex  int
	menuReturn string

	restartRequested  bool
	selectedParentPID uint32
	expandedParents   map[uint32]bool
}

// NewModel creates a new TUI model backed by a StatsService.
func NewModel(statsSvc *service.StatsService, webPort string) Model {
	flows, lastErr := statsSvc.Snapshot()
	history, historyErr := statsSvc.SnapshotHistory()
	if webPort == "" {
		webPort = "N/A"
	}

	model := Model{
		statsSvc:        statsSvc,
		webPort:         webPort,
		flows:           flows,
		history:         history,
		err:             lastErr,
		historyErr:      historyErr,
		mode:            modeRealtime,
		sortBy:          "download",
		sortAsc:         false,
		historySortBy:   historySortDate,
		filterCategory:  "",
		width:           120,
		height:          30,
		activeMenu:      menuNone,
		expandedParents: make(map[uint32]bool),
	}
	model.normalizeViewState()
	return model
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), tea.WindowSize())
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

		if m.activeMenu != menuNone {
			m.handleMenuKey(msg.String())
			if m.quitting {
				return m, tea.Quit
			}
			m.normalizeViewState()
			return m, nil
		}

		switch msg.String() {
		case "q", "Q":
			m.openExitMenu()
		case "tab":
			if m.mode == modeRealtime {
				m.mode = modeHistory
				m.err = m.historyErr
			} else {
				m.mode = modeRealtime
				flows, lastErr := m.statsSvc.Snapshot()
				m.flows = flows
				m.err = lastErr
			}
			m.pageIndex = 0
		case "s", "S":
			if m.mode == modeRealtime {
				m.openSortMenu()
			}
		case "f", "F":
			if m.mode == modeRealtime {
				m.openFilterMenu()
			}
		case "t", "T":
			if m.mode == modeHistory {
				m.historySortBy = historySortTotal
				m.pageIndex = 0
			}
		case "d", "D":
			if m.mode == modeHistory {
				m.historySortBy = historySortDate
				m.pageIndex = 0
			}
		case "left":
			m.movePage(-1)
		case "right":
			m.movePage(1)
		case "up":
			if m.mode == modeRealtime {
				m.moveParentSelection(-1)
			}
		case "down":
			if m.mode == modeRealtime {
				m.moveParentSelection(1)
			}
		case "enter":
			if m.mode == modeRealtime {
				m.toggleSelectedParent()
			}
		}
		m.normalizeViewState()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.normalizeViewState()

	case tickMsg:
		flows, lastErr := m.statsSvc.Snapshot()
		history, historyErr := m.statsSvc.SnapshotHistory()
		m.flows = flows
		m.history = history
		m.err = lastErr
		m.historyErr = historyErr
		if m.mode == modeHistory {
			m.err = historyErr
		} else {
			m.err = lastErr
		}
		m.normalizeViewState()
		return m, tickCmd()
	}

	return m, nil
}

func (m *Model) handleMenuKey(key string) {
	switch key {
	case "esc":
		if m.activeMenu == menuExit && m.menuReturn != menuNone {
			m.activeMenu = m.menuReturn
			m.menuReturn = menuNone
			return
		}
		m.activeMenu = menuNone
		m.menuReturn = menuNone
	case "tab":
		m.activeMenu = menuNone
		m.menuReturn = menuNone
		if m.mode == modeRealtime {
			m.mode = modeHistory
			m.err = m.historyErr
			m.pageIndex = 0
		}
	case "up":
		if m.menuIndex > 0 {
			m.menuIndex--
		}
	case "down":
		maxIndex := m.currentMenuLength() - 1
		if m.menuIndex < maxIndex {
			m.menuIndex++
		}
	case "enter":
		m.applyMenuSelection()
	case "q", "Q":
		if m.activeMenu == menuExit {
			return
		}
		m.openExitMenu()
	case "s", "S":
		if m.activeMenu == menuExit {
			return
		}
		if m.activeMenu == menuSort {
			m.activeMenu = menuNone
		} else if m.mode == modeRealtime {
			m.openSortMenu()
		}
	case "f", "F":
		if m.activeMenu == menuExit {
			return
		}
		if m.activeMenu == menuFilter {
			m.activeMenu = menuNone
		} else if m.mode == modeRealtime {
			m.openFilterMenu()
		}
	}
}

func (m *Model) openExitMenu() {
	if m.activeMenu != menuExit {
		m.menuReturn = m.activeMenu
	}
	m.activeMenu = menuExit
	m.menuIndex = 0
}

func (m *Model) openSortMenu() {
	m.activeMenu = menuSort
	m.menuIndex = m.currentSortMenuIndex()
}

func (m *Model) openFilterMenu() {
	m.activeMenu = menuFilter
	m.menuIndex = m.currentFilterMenuIndex()
}

func (m *Model) applyMenuSelection() {
	switch m.activeMenu {
	case menuSort:
		selected := sortMenuItems[m.menuIndex].key
		if selected == "order" {
			m.sortAsc = !m.sortAsc
		} else {
			m.sortBy = selected
		}
		m.pageIndex = 0
	case menuFilter:
		m.filterCategory = filterMenuItems[m.menuIndex].key
		m.pageIndex = 0
	case menuExit:
		if m.menuIndex == 0 {
			m.quitting = true
		} else {
			m.restartRequested = true
			m.quitting = true
		}
	}
	if !m.quitting {
		m.activeMenu = menuNone
	}
}

func (m Model) currentSortMenuIndex() int {
	for idx, item := range sortMenuItems {
		if item.key == m.sortBy {
			return idx
		}
	}
	return 0
}

func (m Model) currentFilterMenuIndex() int {
	for idx, item := range filterMenuItems {
		if item.key == m.filterCategory {
			return idx
		}
	}
	return 0
}

func (m Model) currentMenuLength() int {
	switch m.activeMenu {
	case menuSort:
		return len(sortMenuItems)
	case menuFilter:
		return len(filterMenuItems)
	case menuExit:
		return 2
	default:
		return 0
	}
}

func (m Model) RestartRequested() bool {
	return m.restartRequested
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) realtimeRows() []processTreeRow {
	return buildProcessTreeRows(m.flows, m.filterCategory, m.sortBy, m.sortAsc, m.expandedParents)
}

func (m Model) realtimePageSize() int {
	return pagedItemsFromHeight(m.height, 13, 2)
}

func (m Model) historyPageSize() int {
	return pagedItemsFromHeight(m.height, 10, 1)
}

func pagedItemsFromHeight(height, reservedLines, linesPerItem int) int {
	pageSize := height - reservedLines
	if pageSize < 6 {
		pageSize = 6
	}
	if linesPerItem <= 1 {
		return pageSize
	}
	items := pageSize / linesPerItem
	if items < 3 {
		return 3
	}
	return items
}

func (m *Model) movePage(delta int) {
	if m.mode == modeHistory {
		m.pageIndex = clampPageIndex(m.pageIndex+delta, len(sortedHistory(m.history, m.historySortBy)), m.historyPageSize())
		return
	}

	m.pageIndex = clampPageIndex(m.pageIndex+delta, len(m.realtimeRows()), m.realtimePageSize())
}

func (m *Model) moveParentSelection(delta int) {
	selectable := m.visibleSelectableParentPIDs()
	if len(selectable) == 0 {
		m.selectedParentPID = 0
		return
	}

	index := 0
	for idx, pid := range selectable {
		if pid == m.selectedParentPID {
			index = idx
			break
		}
	}

	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(selectable) {
		index = len(selectable) - 1
	}
	m.selectedParentPID = selectable[index]
}

func (m *Model) toggleSelectedParent() {
	if m.selectedParentPID == 0 {
		return
	}

	selectable := m.visibleSelectableParentPIDs()
	for _, pid := range selectable {
		if pid != m.selectedParentPID {
			continue
		}
		m.expandedParents[pid] = !m.expandedParents[pid]
		m.normalizeViewState()
		return
	}
}

func (m *Model) normalizeViewState() {
	if m.mode == modeHistory {
		m.pageIndex = clampPageIndex(m.pageIndex, len(sortedHistory(m.history, m.historySortBy)), m.historyPageSize())
		return
	}

	rows := m.realtimeRows()
	m.pageIndex = clampPageIndex(m.pageIndex, len(rows), m.realtimePageSize())
	selectable := m.visibleSelectableParentPIDs()
	if len(selectable) == 0 {
		m.selectedParentPID = 0
		return
	}

	for _, pid := range selectable {
		if pid == m.selectedParentPID {
			return
		}
	}
	m.selectedParentPID = selectable[0]
}

func (m Model) visibleSelectableParentPIDs() []uint32 {
	rows := m.realtimeRows()
	pageRows, _, _ := paginateRows(rows, m.pageIndex, m.realtimePageSize())

	pids := make([]uint32, 0, len(pageRows))
	for _, row := range pageRows {
		if row.Selectable {
			pids = append(pids, row.Flow.PID)
		}
	}
	return pids
}
