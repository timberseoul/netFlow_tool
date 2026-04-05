package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"netFlow_tool-ui/types"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#93C5FD")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#475569"))

	rowStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	dimSpeedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	pidStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A"))
	dimPidStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	nameStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#DDD6FE"))
	childPidStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC"))
	childNameStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#BFDBFE"))
	dimNameStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Italic(true)
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).Bold(true)
	controlStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")).Bold(true)
	activeFilterStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color("#2563EB"))
	inactiveFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	statusActiveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC"))
	statusInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	historyDateStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A"))
	menuBorderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	menuItemStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	menuActiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color("#2563EB")).Bold(true)
	separatorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))
	speedUpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).Bold(true)
	speedDownStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).Bold(true)
	selectedRowStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#1E3A8A"))
	footerHintTagStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#7DD3FC")).Bold(true).Padding(0, 1)
	footerStatsTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#052E16")).Background(lipgloss.Color("#86EFAC")).Bold(true).Padding(0, 1)
	footerPageTagStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#451A03")).Background(lipgloss.Color("#FCD34D")).Bold(true).Padding(0, 1)
)

func (m Model) View() string {
	if m.quitting {
		return "Bye!\n"
	}
	if m.mode == modeHistory {
		return m.renderHistoryView()
	}
	return m.renderRealtimeView()
}

func (m Model) renderRealtimeView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" netFlow_tool — Real-time Network Monitor ") + "\n")
	b.WriteString(renderTopBar(m.mode, renderRealtimeControls(m)) + "\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Connection error: %v", m.err)) + "\n")
		b.WriteString(errorStyle.Render("  Make sure the Rust core is running.") + "\n")
	}

	b.WriteString("\n")

	rows := m.realtimeRows()
	pageRows, pageIndex, totalPages := paginateRows(rows, m.pageIndex, m.realtimePageSize())

	contentWidth := m.width - 2
	if contentWidth < 96 {
		contentWidth = 96
	}

	nameColWidth := contentWidth - 76
	if nameColWidth < 20 {
		nameColWidth = 20
	}
	if nameColWidth > 56 {
		nameColWidth = 56
	}

	headerFmt := fmt.Sprintf("  %%-8s  %%-%ds  %%-8s  %%-8s  %%12s  %%12s  %%10s  %%10s", nameColWidth)
	header := fmt.Sprintf(headerFmt, "PID", "Process", "Status", "Category", "Up Speed", "Down Speed", "Up Total", "Down Total")
	b.WriteString(headerStyle.Render(header) + "\n")

	if len(pageRows) == 0 {
		b.WriteString(rowStyle.Render("  Waiting for network activity...") + "\n")
	} else {
		separatorWidth := contentWidth - 2
		if separatorWidth < 24 {
			separatorWidth = 24
		}
		for _, row := range pageRows {
			processLabel := renderProcessLabel(row, nameColWidth)
			categoryLabel := currentFilterLabel(row.Flow.Category)
			statusLabel := "live"
			statusRenderer := statusActiveStyle
			pidRenderer := pidStyle
			nameRenderer := nameStyle
			upRenderer := speedUpStyle
			downRenderer := speedDownStyle
			totalRenderer := rowStyle

			if row.Flow.Status == "inactive" {
				statusLabel = "idle"
				statusRenderer = statusInactiveStyle
				pidRenderer = dimPidStyle
				nameRenderer = dimNameStyle
				upRenderer = dimSpeedStyle
				downRenderer = dimSpeedStyle
				totalRenderer = dimSpeedStyle
			} else if row.Depth > 0 {
				pidRenderer = childPidStyle
				nameRenderer = childNameStyle
			}

			line := fmt.Sprintf(
				"  %s  %s  %s  %s  %s  %s  %s  %s",
				pidRenderer.Render(fmt.Sprintf("%-8d", row.Flow.PID)),
				nameRenderer.Render(fmt.Sprintf("%-*s", nameColWidth, processLabel)),
				statusRenderer.Render(fmt.Sprintf("%-8s", statusLabel)),
				rowStyle.Render(fmt.Sprintf("%-8s", categoryLabel)),
				upRenderer.Render(fmt.Sprintf("%12s", formatSpeed(row.AggregateUploadSpeed))),
				downRenderer.Render(fmt.Sprintf("%12s", formatSpeed(row.AggregateDownloadSpeed))),
				totalRenderer.Render(fmt.Sprintf("%10s", formatBytes(row.AggregateTotalUpload))),
				totalRenderer.Render(fmt.Sprintf("%10s", formatBytes(row.AggregateTotalDownload))),
			)

			if row.Selectable && row.Flow.PID == m.selectedParentPID {
				line = selectedRowStyle.Render(line)
			}
			b.WriteString(line + "\n")
			b.WriteString(separatorStyle.Render("  "+strings.Repeat("-", separatorWidth)) + "\n")
		}
	}

	var totalUp, totalDown float64
	var totalUpBytes, totalDownBytes uint64
	visibleCount := 0
	for _, flow := range m.flows {
		if m.filterCategory != "" && flow.Category != m.filterCategory {
			continue
		}
		totalUp += flow.UploadSpeed
		totalDown += flow.DownloadSpeed
		totalUpBytes += flow.TotalUpload
		totalDownBytes += flow.TotalDownload
		visibleCount++
	}

	b.WriteString("\n")
	b.WriteString(footerHintTagStyle.Render(" 快捷键提示 ") + " " + helpStyle.Render(renderRealtimeHelp(m.activeMenu)) + "\n")
	b.WriteString(footerStatsTagStyle.Render(" 数据流统计 ") + " " + rowStyle.Render(fmt.Sprintf("Speed: ↑ %s  ↓ %s  |  Traffic: ↑ %s  ↓ %s  |  Visible: %d  |  WebUI Port: %s", formatSpeed(totalUp), formatSpeed(totalDown), formatBytes(totalUpBytes), formatBytes(totalDownBytes), visibleCount, m.webPort)) + "\n")
	b.WriteString(footerPageTagStyle.Render(" 页面显示 ") + " " + rowStyle.Render(fmt.Sprintf("Current Page: %d/%d  |  Items/Page: %d", pageIndex+1, totalPages, m.realtimePageSize())) + "\n")

	return m.applyMenuOverlay(b.String())
}

func (m Model) renderHistoryView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" netFlow_tool — Daily Traffic History ") + "\n")
	b.WriteString(renderTopBar(m.mode, renderHistoryControls(m)) + "\n")

	if m.historyErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  History load error: %v", m.historyErr)) + "\n")
		b.WriteString(errorStyle.Render("  Showing the last successfully loaded history cache.") + "\n")
	}

	b.WriteString("\n")

	rows := sortedHistory(m.history, m.historySortBy)
	pageRows, pageIndex, totalPages := paginateRows(rows, m.pageIndex, m.historyPageSize())
	header := fmt.Sprintf("  %-12s %-12s %-12s %-12s", "Date", "Upload", "Download", "Total")
	b.WriteString(headerStyle.Render(header) + "\n")

	if len(pageRows) == 0 {
		b.WriteString(rowStyle.Render("  No saved history yet.") + "\n")
	} else {
		for _, row := range pageRows {
			total := row.Upload + row.Download
			b.WriteString(fmt.Sprintf(
				"  %s %12s %12s %12s\n",
				historyDateStyle.Render(fmt.Sprintf("%-12s", formatHistoryDate(row.Date))),
				speedUpStyle.Render(fmt.Sprintf("%12s", formatBytes(row.Upload))),
				speedDownStyle.Render(fmt.Sprintf("%12s", formatBytes(row.Download))),
				rowStyle.Render(fmt.Sprintf("%12s", formatBytes(total))),
			))
		}
	}

	var totalUpload, totalDownload uint64
	for _, row := range rows {
		totalUpload += row.Upload
		totalDownload += row.Download
	}

	b.WriteString("\n")
	b.WriteString(footerHintTagStyle.Render(" 快捷键提示 ") + " " + helpStyle.Render(fmt.Sprintf("[Left/Right] Page  [T] Total desc  [D] Date desc  [Tab] Realtime  [Q] Exit menu  |  Current: %s", currentHistorySortLabel(m.historySortBy))) + "\n")
	b.WriteString(footerStatsTagStyle.Render(" 数据流统计 ") + " " + rowStyle.Render(fmt.Sprintf("Days: %d  |  Total Upload: %s  |  Total Download: %s  |  WebUI Port: %s", len(rows), formatBytes(totalUpload), formatBytes(totalDownload), m.webPort)) + "\n")
	b.WriteString(footerPageTagStyle.Render(" 页面显示 ") + " " + rowStyle.Render(fmt.Sprintf("Current Page: %d/%d  |  Items/Page: %d", pageIndex+1, totalPages, m.historyPageSize())) + "\n")

	return m.applyMenuOverlay(b.String())
}

func renderProcessLabel(row processTreeRow, width int) string {
	indent := strings.Repeat("  ", row.Depth)
	prefix := "  "
	switch {
	case row.HasChildren && row.Expanded:
		prefix = "v "
	case row.HasChildren:
		prefix = "> "
	case row.Depth > 0:
		prefix = "|-"
	}

	label := indent + prefix + row.Flow.Name
	if len(label) > width {
		if width <= 3 {
			return label[:width]
		}
		return label[:width-3] + "..."
	}
	return label
}

func renderRealtimeControls(m Model) string {
	sortLabel := fmt.Sprintf("Sort [S]: %s / %s", currentSortLabel(m.sortBy), currentSortDirection(m.sortAsc))
	filterLabel := fmt.Sprintf("Filter [F]: %s", currentFilterLabel(m.filterCategory))

	if m.activeMenu == menuSort {
		sortLabel = activeFilterStyle.Render(" " + sortLabel + " ")
	} else {
		sortLabel = controlStyle.Render("  " + sortLabel)
	}

	if m.activeMenu == menuFilter {
		filterLabel = activeFilterStyle.Render(" " + filterLabel + " ")
	} else {
		filterLabel = controlStyle.Render("  " + filterLabel)
	}

	return sortLabel + "    " + filterLabel
}

func renderHistoryControls(m Model) string {
	return controlStyle.Render(fmt.Sprintf("  Sort [T/D]: %s", currentHistorySortLabel(m.historySortBy)))
}

func renderTopBar(mode, controls string) string {
	return "  " + renderModeTabs(mode) + "    " + controls
}

func renderKeyboardMenu(title string, items []string, selected int) []string {
	lines := []string{
		menuBorderStyle.Render("  +" + strings.Repeat("-", sortMenuWidth) + "+"),
		menuBorderStyle.Render("  |" + padMenuItem(title, sortMenuWidth) + "|"),
	}
	for idx, item := range items {
		style := menuItemStyle
		prefix := "  "
		if idx == selected {
			style = menuActiveStyle
			prefix = "> "
		}
		lines = append(lines, menuBorderStyle.Render("  |")+style.Render(padMenuItem(prefix+item, sortMenuWidth))+menuBorderStyle.Render("|"))
	}
	lines = append(lines, menuBorderStyle.Render("  +"+strings.Repeat("-", sortMenuWidth)+"+"))
	return lines
}

func buildSortMenuLabels(m Model) []string {
	labels := make([]string, 0, len(sortMenuItems))
	for _, item := range sortMenuItems {
		if item.key == "order" {
			labels = append(labels, fmt.Sprintf("Order: %s", currentSortDirection(m.sortAsc)))
			continue
		}
		labels = append(labels, item.label)
	}
	return labels
}

func buildFilterMenuLabels(_ Model) []string {
	labels := make([]string, 0, len(filterMenuItems))
	for _, item := range filterMenuItems {
		labels = append(labels, item.label)
	}
	return labels
}

func buildExitMenuLabels() []string {
	return []string{
		"Quit",
		"Restart",
	}
}

func renderRealtimeHelp(activeMenu string) string {
	if activeMenu == menuExit {
		return "  [Up/Down] Select  [Enter] Confirm  [Esc] Back  [Tab] History  [Ctrl+C] Force Quit"
	}
	if activeMenu == menuSort || activeMenu == menuFilter {
		return "  [Up/Down] Select  [Enter] Confirm  [Esc] Close  [Q] Exit Menu  [Tab] History"
	}
	return "  [Left/Right] Page  [Up/Down] Parent Select  [Enter] Toggle Children  [S] Sort  [F] Filter  [Tab] History  [Q] Exit menu"
}

func (m Model) applyMenuOverlay(base string) string {
	if m.activeMenu == menuNone {
		return base
	}

	overlayLines := m.activeMenuLines()
	baseLines := strings.Split(base, "\n")
	startRow := 2
	for len(baseLines) < startRow+len(overlayLines) {
		baseLines = append(baseLines, "")
	}

	for i, line := range overlayLines {
		baseLines[startRow+i] = line
	}

	return strings.Join(baseLines, "\n")
}

func (m Model) activeMenuLines() []string {
	switch m.activeMenu {
	case menuSort:
		lines := renderKeyboardMenu("Sort Menu", buildSortMenuLabels(m), m.menuIndex)
		return append(lines, helpStyle.Render("  Up/Down to select, Enter to apply, Esc to close"))
	case menuFilter:
		lines := renderKeyboardMenu("Filter Menu", buildFilterMenuLabels(m), m.menuIndex)
		return append(lines, helpStyle.Render("  Up/Down to select, Enter to apply, Esc to close"))
	case menuExit:
		lines := renderKeyboardMenu("Exit Menu", buildExitMenuLabels(), m.menuIndex)
		return append(lines, helpStyle.Render("  Up/Down to select, Enter to confirm, Esc to go back"))
	default:
		return nil
	}
}

func formatHistoryDate(date string) string {
	if len(date) >= 10 {
		return date[:10]
	}
	return date
}

func renderModeTabs(mode string) string {
	realtime := inactiveFilterStyle.Render(" [Realtime] ")
	history := inactiveFilterStyle.Render(" [History] ")
	if mode == modeRealtime {
		realtime = activeFilterStyle.Render(" [Realtime] ")
	} else {
		history = activeFilterStyle.Render(" [History] ")
	}
	return realtime + " -> " + history
}

func padMenuItem(label string, width int) string {
	current := lipgloss.Width(label)
	if current >= width {
		return label
	}
	return label + strings.Repeat(" ", width-current)
}

func currentHistorySortLabel(sortBy string) string {
	if sortBy == historySortTotal {
		return "Total"
	}
	return "Date"
}

func sortedHistory(rows []types.DailyUsage, sortBy string) []types.DailyUsage {
	result := make([]types.DailyUsage, len(rows))
	copy(result, rows)
	sort.Slice(result, func(i, j int) bool {
		if sortBy == historySortTotal {
			leftTotal := result[i].Upload + result[i].Download
			rightTotal := result[j].Upload + result[j].Download
			if leftTotal == rightTotal {
				return result[j].Date < result[i].Date
			}
			return leftTotal > rightTotal
		}
		return result[j].Date < result[i].Date
	})
	return result
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
	if filter == "unknown" {
		return "Unknown"
	}
	return "All"
}

func currentSortDirection(sortAsc bool) string {
	if sortAsc {
		return "Asc"
	}
	return "Desc"
}

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
