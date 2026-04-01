package ui

import (
	"fmt"
	"sort"
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
	modeRealtime    = "realtime"
	modeHistory     = "history"
	scrollWheelStep = 3
	sortMenuWidth   = 24

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
	statsSvc       *service.StatsService
	webPort        string
	flows          []types.ProcessFlow
	history        []types.DailyUsage
	err            error
	historyErr     error
	mode           string
	width          int
	height         int
	sortBy         string // "download", "upload", "name", "pid"
	sortAsc        bool
	historySortBy  string
	quitting       bool
	filterCategory string // "" = all, "user", "system", "service"
	scrollOffset   int    // vertical scroll position (0 = top)
	totalRows      int    // total renderable rows (for scroll bounds)

	activeMenu string
	menuIndex  int
	menuReturn string

	restartRequested bool

	draggingScrollbar  bool
	scrollbarDragDelta int
}

// NewModel creates a new TUI model backed by a StatsService.
func NewModel(statsSvc *service.StatsService, webPort string) Model {
	// Read initial data so the first frame has content.
	flows, lastErr := statsSvc.Snapshot()
	history, historyErr := statsSvc.SnapshotHistory()
	if webPort == "" {
		webPort = "N/A"
	}
	return Model{
		statsSvc:       statsSvc,
		webPort:        webPort,
		flows:          flows,
		history:        history,
		err:            lastErr,
		historyErr:     historyErr,
		mode:           modeRealtime,
		sortBy:         "download",
		sortAsc:        false,
		historySortBy:  historySortDate,
		filterCategory: "",
		width:          120,
		height:         30,
		activeMenu:     menuNone,
	}
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
			m.clampScroll()
			return m, nil
		}

		switch msg.String() {
		case "q", "Q":
			m.openExitMenu()
		case "tab":
			if m.mode == modeRealtime {
				m.mode = modeHistory
				m.err = m.historyErr
				m.activeMenu = menuNone
			} else {
				m.mode = modeRealtime
				flows, lastErr := m.statsSvc.Snapshot()
				m.flows = flows
				m.err = lastErr
			}
			m.scrollOffset = 0
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
				m.scrollOffset = 0
			}
		case "d", "D":
			if m.mode == modeHistory {
				m.historySortBy = historySortDate
				m.scrollOffset = 0
			}
		}
		m.clampScroll()

	case tea.MouseMsg:
		m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()

	case tickMsg:
		// Read the latest cached stats from the background service.
		// This is non-blocking — no IPC happens here.
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
		m.totalRows = m.totalContentRows()
		m.clampScroll()
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
			m.scrollOffset = 0
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
	case menuFilter:
		m.filterCategory = filterMenuItems[m.menuIndex].key
		m.scrollOffset = 0
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

// tickCmd schedules the next UI refresh.
// This fires at a true fixed 1-second interval regardless of IPC latency.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// sortFlows sorts the flow list based on current sort settings.
func (m *Model) sortFlows() []types.ProcessFlow {
	flows := make([]types.ProcessFlow, len(m.flows))
	copy(flows, m.flows)

	sort.Slice(flows, func(i, j int) bool {
		var less bool
		switch m.sortBy {
		case "download":
			less = flows[i].DownloadSpeed > flows[j].DownloadSpeed
		case "upload":
			less = flows[i].UploadSpeed > flows[j].UploadSpeed
		case "name":
			less = flows[i].Name < flows[j].Name
		case "pid":
			less = flows[i].PID < flows[j].PID
		default:
			less = flows[i].DownloadSpeed > flows[j].DownloadSpeed
		}
		if m.sortAsc {
			return !less
		}
		return less
	})

	return flows
}

func (m *Model) handleMouse(msg tea.MouseMsg) {
	switch {
	case msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress:
		m.scrollBy(-scrollWheelStep)
	case msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress:
		m.scrollBy(scrollWheelStep)
	case msg.Button == tea.MouseButtonLeft:
		layout := m.currentScrollLayout()
		if layout.total <= layout.height {
			if msg.Action == tea.MouseActionRelease {
				m.draggingScrollbar = false
			}
			return
		}

		switch msg.Action {
		case tea.MouseActionPress:
			if !m.isScrollbarHit(msg.X, msg.Y, layout) {
				m.draggingScrollbar = false
				return
			}

			row := msg.Y - layout.top
			if row >= layout.thumbStart && row < layout.thumbStart+layout.thumbHeight {
				m.draggingScrollbar = true
				m.scrollbarDragDelta = row - layout.thumbStart
				return
			}

			targetThumbStart := row - layout.thumbHeight/2
			m.scrollOffset = scrollOffsetFromThumb(layout.total, layout.height, targetThumbStart)
			m.clampScroll()

			updatedLayout := m.currentScrollLayout()
			m.draggingScrollbar = true
			m.scrollbarDragDelta = row - updatedLayout.thumbStart
			if m.scrollbarDragDelta < 0 {
				m.scrollbarDragDelta = 0
			}
			if m.scrollbarDragDelta >= updatedLayout.thumbHeight {
				m.scrollbarDragDelta = updatedLayout.thumbHeight - 1
			}
		case tea.MouseActionMotion:
			if !m.draggingScrollbar {
				return
			}
			targetThumbStart := msg.Y - layout.top - m.scrollbarDragDelta
			m.scrollOffset = scrollOffsetFromThumb(layout.total, layout.height, targetThumbStart)
			m.clampScroll()
		case tea.MouseActionRelease:
			m.draggingScrollbar = false
		}
	case msg.Action == tea.MouseActionRelease:
		m.draggingScrollbar = false
	}
}

func (m *Model) scrollBy(delta int) {
	m.scrollOffset += delta
	m.clampScroll()
}

func (m *Model) clampScroll() {
	layout := m.currentScrollLayout()
	m.scrollOffset = layout.offset
	m.totalRows = layout.total
}

func (m Model) currentScrollLayout() scrollLayout {
	return newScrollLayout(m.contentTopLine(), m.viewportHeight(), m.totalContentRows(), m.scrollOffset)
}

func (m Model) viewportHeight() int {
	return m.height - m.contentTopLine() - m.bottomReservedLines()
}

func (m Model) contentTopLine() int {
	if m.mode == modeHistory {
		top := 5
		if m.historyErr != nil {
			top += 3
		}
		return top
	}

	top := 6
	if m.err != nil {
		top += 3
	}
	if m.activeMenu == menuSort {
		top += len(sortMenuItems) + 2
	}
	if m.activeMenu == menuFilter {
		top += len(filterMenuItems) + 2
	}
	return top
}

func (m Model) bottomReservedLines() int {
	return 4
}

func (m Model) totalContentRows() int {
	if m.mode == modeHistory {
		rows := len(m.history)
		if rows == 0 {
			return 1
		}
		return rows * 2
	}

	counts := make(map[string]int)
	for _, f := range m.flows {
		cat := f.Category
		if cat == "" {
			cat = "unknown"
		}
		if m.filterCategory != "" && cat != m.filterCategory {
			continue
		}
		counts[cat]++
	}

	total := 0
	for _, cat := range categoryOrder {
		if counts[cat.key] == 0 {
			continue
		}
		total += 2 + counts[cat.key]*2
	}

	if total == 0 {
		return 1
	}
	return total
}

func (m Model) isScrollbarHit(x, y int, layout scrollLayout) bool {
	if y < layout.top || y >= layout.top+layout.height {
		return false
	}
	return x >= scrollbarHitMinX(m.width)
}

func currentSortLabel(sortBy string) string {
	for _, item := range sortMenuItems {
		if item.key == sortBy {
			return item.label
		}
	}
	return "Download"
}

func currentFilterLabel(filter string) string {
	for _, item := range filterMenuItems {
		if item.key == filter {
			return item.label
		}
	}
	return "All"
}

func currentSortDirection(sortAsc bool) string {
	if sortAsc {
		return "Asc"
	}
	return "Desc"
}

// formatSpeed converts bytes/sec to a human-readable string.
func formatSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB/s", bytesPerSec/(1024*1024*1024))
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/1024)
	default:
		return fmt.Sprintf("%.0f  B/s", bytesPerSec)
	}
}

// formatBytes converts a byte count to a human-readable string.
func formatBytes(bytes uint64) string {
	b := float64(bytes)
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", b/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.2f MB", b/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", b/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
