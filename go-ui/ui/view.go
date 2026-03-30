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
	dimNameStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	helpStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Italic(true)
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).Bold(true)
	controlStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")).Bold(true)
	categoryHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#93C5FD")).PaddingLeft(1)
	activeFilterStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color("#2563EB"))
	inactiveFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	statusActiveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC"))
	statusInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	historyDateStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A"))
	scrollbarTrackStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	scrollbarThumbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true)
	menuBorderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	menuItemStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	menuActiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color("#2563EB")).Bold(true)
	separatorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))
	speedUpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).Bold(true)
	speedDownStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).Bold(true)
)

var categoryOrder = []struct {
	key   string
	label string
}{
	{"user", "User Processes"},
	{"system", "System Processes"},
	{"service", "Services"},
}

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
	b.WriteString(titleStyle.Render(" netFlow_tool — Real-time Network Monitor ") + "\n\n")
	b.WriteString(renderModeTabs(m.mode) + "\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Connection error: %v", m.err)) + "\n")
		b.WriteString(errorStyle.Render("  Make sure the Rust core is running.") + "\n\n")
	}

	b.WriteString(renderRealtimeControls(m) + "\n")
	if m.activeMenu == menuSort {
		for _, line := range renderKeyboardMenu("Sort Menu", buildSortMenuLabels(m), m.menuIndex) {
			b.WriteString(line + "\n")
		}
	}
	if m.activeMenu == menuFilter {
		for _, line := range renderKeyboardMenu("Filter Menu", buildFilterMenuLabels(m), m.menuIndex) {
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")

	flows := m.sortFlows()
	grouped := make(map[string][]types.ProcessFlow)
	for _, f := range flows {
		cat := f.Category
		if cat == "" {
			cat = "unknown"
		}
		if m.filterCategory != "" && cat != m.filterCategory {
			continue
		}
		grouped[cat] = append(grouped[cat], f)
	}

	contentWidth := m.width - 2
	if contentWidth < 80 {
		contentWidth = 80
	}

	nameColWidth := 16
	for _, catFlows := range grouped {
		for _, f := range catFlows {
			if len(f.Name) > nameColWidth {
				nameColWidth = len(f.Name)
			}
		}
	}
	if nameColWidth > 60 {
		nameColWidth = 60
	}
	maxNameColWidth := contentWidth - 74
	if maxNameColWidth < 8 {
		maxNameColWidth = 8
	}
	if nameColWidth > maxNameColWidth {
		nameColWidth = maxNameColWidth
	}

	headerFmt := fmt.Sprintf("  %%-8s  %%-%ds  %%-8s  %%12s  %%12s  %%10s  %%10s", nameColWidth)
	header := fmt.Sprintf(headerFmt, "PID", "Process", "Status", "Up Speed", "Down Speed", "Up Total", "Down Total")
	b.WriteString(headerStyle.Render(header) + "\n")

	var contentLines []string
	hasAny := false
	for _, cat := range categoryOrder {
		if len(grouped[cat.key]) > 0 {
			hasAny = true
			break
		}
	}

	separatorWidth := contentWidth - 4
	if separatorWidth < 20 {
		separatorWidth = 20
	}
	rowFmtName := fmt.Sprintf("%%-%ds", nameColWidth)

	if !hasAny {
		contentLines = append(contentLines, rowStyle.Render("  Waiting for network activity..."))
	} else {
		for _, cat := range categoryOrder {
			catFlows, ok := grouped[cat.key]
			if !ok || len(catFlows) == 0 {
				continue
			}

			contentLines = append(contentLines, "")
			contentLines = append(contentLines, categoryHeaderStyle.Render(fmt.Sprintf("-- %s (%d) --", cat.label, len(catFlows))))

			for _, f := range catFlows {
				name := f.Name
				if len(name) > nameColWidth {
					name = name[:nameColWidth-3] + "..."
				}

				var row string
				if f.Status == "inactive" {
					row = fmt.Sprintf(
						"  %s  %s  %s  %s  %s  %s  %s",
						dimPidStyle.Render(fmt.Sprintf("%-8d", f.PID)),
						dimNameStyle.Render(fmt.Sprintf(rowFmtName, name)),
						statusInactiveStyle.Render(fmt.Sprintf("%-8s", "idle")),
						dimSpeedStyle.Render(fmt.Sprintf("%12s", "--")),
						dimSpeedStyle.Render(fmt.Sprintf("%12s", "--")),
						dimSpeedStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalUpload))),
						dimSpeedStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalDownload))),
					)
				} else {
					row = fmt.Sprintf(
						"  %s  %s  %s  %s  %s  %s  %s",
						pidStyle.Render(fmt.Sprintf("%-8d", f.PID)),
						nameStyle.Render(fmt.Sprintf(rowFmtName, name)),
						statusActiveStyle.Render(fmt.Sprintf("%-8s", "live")),
						speedUpStyle.Render(fmt.Sprintf("%12s", formatSpeed(f.UploadSpeed))),
						speedDownStyle.Render(fmt.Sprintf("%12s", formatSpeed(f.DownloadSpeed))),
						rowStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalUpload))),
						rowStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalDownload))),
					)
				}
				contentLines = append(contentLines, row)
				contentLines = append(contentLines, separatorStyle.Render("  "+strings.Repeat("─", separatorWidth)))
			}
		}
	}

	scrollOff, endIdx, viewportHeight := clampViewport(m.scrollOffset, len(contentLines), m.viewportHeight())
	for idx, line := range contentLines[scrollOff:endIdx] {
		line = renderScrollableLine(line, idx, len(contentLines), viewportHeight, scrollOff, m.width)
		b.WriteString(line + "\n")
	}

	var totalUp, totalDown float64
	var totalUpBytes, totalDownBytes uint64
	count := 0
	for _, catFlows := range grouped {
		for _, f := range catFlows {
			totalUp += f.UploadSpeed
			totalDown += f.DownloadSpeed
			totalUpBytes += f.TotalUpload
			totalDownBytes += f.TotalDownload
			count++
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(renderRealtimeHelp(m.activeMenu)) + "\n")
	b.WriteString(rowStyle.Render(fmt.Sprintf("  Speed: ↑ %s  ↓ %s  |  Traffic: ↑ %s  ↓ %s  |  %d processes", formatSpeed(totalUp), formatSpeed(totalDown), formatBytes(totalUpBytes), formatBytes(totalDownBytes), count)) + "\n")

	return b.String()
}

func (m Model) renderHistoryView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" netFlow_tool — Daily Traffic History ") + "\n\n")
	b.WriteString(renderModeTabs(m.mode) + "\n")

	if m.historyErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  History load error: %v", m.historyErr)) + "\n")
		b.WriteString(errorStyle.Render("  Showing the last successfully loaded history cache.") + "\n\n")
	}

	rows := sortedHistory(m.history, m.historySortBy)
	header := fmt.Sprintf("  %-12s %-10s %-10s %-10s", "Date", "Upload", "Download", "Total")
	b.WriteString(headerStyle.Render(header) + "\n")
	b.WriteString(helpStyle.Render("  --------------------------------------------------------") + "\n")

	var contentLines []string
	if len(rows) == 0 {
		contentLines = append(contentLines, rowStyle.Render("  No saved history yet."))
	} else {
		for _, row := range rows {
			total := row.Upload + row.Download
			contentLines = append(contentLines, fmt.Sprintf(
				"  %s %10s %10s %10s",
				historyDateStyle.Render(fmt.Sprintf("%-12s", formatHistoryDate(row.Date))),
				speedUpStyle.Render(fmt.Sprintf("%10s", formatBytes(row.Upload))),
				speedDownStyle.Render(fmt.Sprintf("%10s", formatBytes(row.Download))),
				rowStyle.Render(fmt.Sprintf("%10s", formatBytes(total))),
			))
			contentLines = append(contentLines, helpStyle.Render("  --------------------------------------------------------"))
		}
	}

	scrollOff, endIdx, viewportHeight := clampViewport(m.scrollOffset, len(contentLines), m.viewportHeight())
	for idx, line := range contentLines[scrollOff:endIdx] {
		line = renderScrollableLine(line, idx, len(contentLines), viewportHeight, scrollOff, m.width)
		b.WriteString(line + "\n")
	}

	var totalUpload, totalDownload uint64
	for _, row := range rows {
		totalUpload += row.Upload
		totalDownload += row.Download
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("  [T] Total desc  [D] Date desc  [Tab] Realtime  [Q] Quit  |  Current: %s", currentHistorySortLabel(m.historySortBy))) + "\n")
	b.WriteString(rowStyle.Render(fmt.Sprintf("  Days: %d  |  Total Upload: %s  |  Total Download: %s", len(rows), formatBytes(totalUpload), formatBytes(totalDownload))) + "\n")

	return b.String()
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

func renderRealtimeHelp(activeMenu string) string {
	if activeMenu == menuSort || activeMenu == menuFilter {
		return "  [Up/Down] Select  [Enter] Confirm  [Esc] Close  [Tab] History  [Q] Quit"
	}
	return "  [S] Sort menu  [F] Filter menu  [Tab] History  [Q] Quit"
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
	return "  " + realtime + " ↔ " + history
}

func clampViewport(offset, total, desiredHeight int) (int, int, int) {
	desiredHeight = clampViewportHeight(desiredHeight)
	offset = clampScrollOffset(offset, total, desiredHeight)
	end := offset + desiredHeight
	if end > total {
		end = total
	}
	return offset, end, desiredHeight
}

func renderScrollableLine(line string, rowIndex, total, viewportHeight, offset, width int) string {
	contentWidth := width - 2
	if contentWidth < 1 {
		return line
	}

	line = padVisibleWidth(line, contentWidth)
	if total <= viewportHeight {
		return line
	}

	thumbStart, thumbHeight := scrollbarThumb(total, viewportHeight, offset)
	glyph := scrollbarTrackStyle.Render("│")
	if rowIndex >= thumbStart && rowIndex < thumbStart+thumbHeight {
		glyph = scrollbarThumbStyle.Render("█")
	}

	return line + " " + glyph
}

func padVisibleWidth(line string, width int) string {
	current := lipgloss.Width(line)
	if current >= width {
		return line
	}
	return line + strings.Repeat(" ", width-current)
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
