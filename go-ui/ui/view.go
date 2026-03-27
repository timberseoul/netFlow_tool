package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"netFlow_tool-ui/types"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF88")).
			Background(lipgloss.Color("#1a1a2e")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3B4048"))

	rowStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#ABB2BF"))
	inactiveRowStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	speedUpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75"))
	speedDownStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379"))
	dimSpeedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5263"))
	pidStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B"))
	dimPidStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	nameStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#C678DD"))
	dimNameStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	helpStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Italic(true)
	errorStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")).Bold(true)
	sortIndicator        = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF88")).Bold(true)
	categoryHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#61AFEF")).PaddingLeft(1)
	activeFilterStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FF88"))
	inactiveFilterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	statusActiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379"))
	statusInactiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370"))
	historyDateStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B"))
	historyUsageBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
)

var categoryOrder = []struct {
	key   string
	label string
}{
	{"user", "👤 User Processes"},
	{"system", "⚙ System Processes"},
	{"service", "🔧 Services"},
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
	b.WriteString(titleStyle.Render(" ⚡ netFlow_tool — Real-time Network Monitor ") + "\n\n")
	b.WriteString(renderModeTabs(m.mode) + "\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ⚠  Connection error: %v", m.err)) + "\n")
		b.WriteString(errorStyle.Render("     Make sure the Rust core is running.") + "\n\n")
	}

	filterLine := "  Filter: "
	filters := []struct {
		key   string
		label string
	}{
		{"", "All[1]"},
		{"user", "User[2]"},
		{"system", "System[3]"},
		{"service", "Service[4]"},
	}
	for _, f := range filters {
		if m.filterCategory == f.key {
			filterLine += activeFilterStyle.Render(" "+f.label+" ") + " "
		} else {
			filterLine += inactiveFilterStyle.Render(" "+f.label+" ") + " "
		}
	}
	b.WriteString(filterLine + "\n")

	sortLabel := fmt.Sprintf("  Sort: %s", m.sortBy)
	if m.sortAsc {
		sortLabel += " ▲"
	} else {
		sortLabel += " ▼"
	}
	b.WriteString(sortIndicator.Render(sortLabel) + "\n\n")

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

	headerFmt := fmt.Sprintf("  %%-8s  %%-%ds  %%-8s  %%12s  %%12s  %%10s  %%10s", nameColWidth)
	header := fmt.Sprintf(headerFmt, "PID", "Process", "Status", "↑ Speed", "↓ Speed", "↑ Total", "↓ Total")
	b.WriteString(headerStyle.Render(header) + "\n")

	var contentLines []string
	hasAny := false
	for _, cat := range categoryOrder {
		if len(grouped[cat.key]) > 0 {
			hasAny = true
			break
		}
	}

	if !hasAny {
		contentLines = append(contentLines, rowStyle.Render("  Waiting for network activity..."))
	} else {
		rowFmtName := fmt.Sprintf("%%-%ds", nameColWidth)
		for _, cat := range categoryOrder {
			catFlows, ok := grouped[cat.key]
			if !ok || len(catFlows) == 0 {
				continue
			}

			contentLines = append(contentLines, "")
			contentLines = append(contentLines, categoryHeaderStyle.Render(fmt.Sprintf("── %s (%d) ──", cat.label, len(catFlows))))

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
						dimSpeedStyle.Render(fmt.Sprintf("%12s", "—")),
						dimSpeedStyle.Render(fmt.Sprintf("%12s", "—")),
						dimSpeedStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalUpload))),
						dimSpeedStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalDownload))),
					)
				} else {
					row = fmt.Sprintf(
						"  %s  %s  %s  %s  %s  %s  %s",
						pidStyle.Render(fmt.Sprintf("%-8d", f.PID)),
						nameStyle.Render(fmt.Sprintf(rowFmtName, name)),
						statusActiveStyle.Render(fmt.Sprintf("%-8s", "●")),
						speedUpStyle.Render(fmt.Sprintf("%12s", formatSpeed(f.UploadSpeed))),
						speedDownStyle.Render(fmt.Sprintf("%12s", formatSpeed(f.DownloadSpeed))),
						rowStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalUpload))),
						rowStyle.Render(fmt.Sprintf("%10s", formatBytes(f.TotalDownload))),
					)
				}
				contentLines = append(contentLines, row)
			}
		}
	}

	scrollOff, endIdx, viewportHeight := clampViewport(m.scrollOffset, len(contentLines), m.height-12)
	for _, line := range contentLines[scrollOff:endIdx] {
		b.WriteString(line + "\n")
	}

	if len(contentLines) > viewportHeight {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ── ↑↓/j/k scroll | showing %d-%d of %d rows ──", scrollOff+1, endIdx, len(contentLines))) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [tab] 历史模式  [s] Sort speed  [n] Name  [p] PID  [r] Reverse  [1-4] Filter  [↑↓] Scroll  [q] Quit") + "\n")

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
	b.WriteString("\n" + rowStyle.Render(fmt.Sprintf("  Speed: ↑ %s  ↓ %s  |  Traffic: ↑ %s  ↓ %s  |  %d processes", formatSpeed(totalUp), formatSpeed(totalDown), formatBytes(totalUpBytes), formatBytes(totalDownBytes), count)) + "\n")

	return b.String()
}

func (m Model) renderHistoryView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" ⚡ netFlow_tool — Daily Traffic History ") + "\n\n")
	b.WriteString(renderModeTabs(m.mode) + "\n")

	if m.historyErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ⚠  History load error: %v", m.historyErr)) + "\n")
		b.WriteString(errorStyle.Render("     Showing the last successfully loaded history cache.") + "\n\n")
	}

	rows := sortedHistory(m.history)
	barWidth := 50

	maxUsage := uint64(0)
	for _, row := range rows {
		usage := row.Upload + row.Download
		if usage > maxUsage {
			maxUsage = usage
		}
	}

	header := fmt.Sprintf("  %-10s  %-10s  %-10s  %-50s", "Date", "Upload", "Download", "Usage")
	b.WriteString(headerStyle.Render(header) + "\n")

	var contentLines []string
	if len(rows) == 0 {
		contentLines = append(contentLines, rowStyle.Render("  No saved history yet."))
	} else {
		for _, row := range rows {
			usage := row.Upload + row.Download
			contentLines = append(contentLines, fmt.Sprintf(
				"  %s  %10s  %10s  %s",
				historyDateStyle.Render(fmt.Sprintf("%-10s", formatHistoryDate(row.Date))),
				speedUpStyle.Render(fmt.Sprintf("%10s", formatBytes(row.Upload))),
				speedDownStyle.Render(fmt.Sprintf("%10s", formatBytes(row.Download))),
				renderUsageBar(usage, maxUsage, barWidth),
			))
		}
	}

	scrollOff, endIdx, viewportHeight := clampViewport(m.scrollOffset, len(contentLines), m.height-10)
	for _, line := range contentLines[scrollOff:endIdx] {
		b.WriteString(line + "\n")
	}

	if len(contentLines) > viewportHeight {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ── ↑↓/j/k scroll | showing %d-%d of %d rows ──", scrollOff+1, endIdx, len(contentLines))) + "\n")
	}

	var totalUpload, totalDownload uint64
	for _, row := range rows {
		totalUpload += row.Upload
		totalDownload += row.Download
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [tab] 实时模式  [↑↓] Scroll  [q] Quit") + "\n")
	b.WriteString("\n" + rowStyle.Render(fmt.Sprintf("  Days: %d  |  Total Upload: %s  |  Total Download: %s", len(rows), formatBytes(totalUpload), formatBytes(totalDownload))) + "\n")

	return b.String()
}

func formatHistoryDate(date string) string {
	if len(date) >= 10 {
		return date[5:10]
	}
	return date
}

func renderModeTabs(mode string) string {
	realtime := inactiveFilterStyle.Render(" [实时模式] ")
	history := inactiveFilterStyle.Render(" [历史模式] ")
	if mode == modeRealtime {
		realtime = activeFilterStyle.Render(" [实时模式] ")
	} else {
		history = activeFilterStyle.Render(" [历史模式] ")
	}
	return "  " + realtime + " ↔ " + history
}

func renderBar(value, max uint64, width int, style lipgloss.Style) string {
	if width <= 0 || max == 0 || value == 0 {
		return ""
	}
	ratio := float64(value) / float64(max)
	filled := int(ratio * float64(width))
	if filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return style.Render(strings.Repeat("█", filled))
}

func renderUsageBar(value, max uint64, width int) string {
	return renderBar(value, max, width, historyUsageBarStyle)
}

func clampViewport(offset, total, desiredHeight int) (int, int, int) {
	if desiredHeight < 5 {
		desiredHeight = 5
	}
	maxScroll := total - desiredHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxScroll {
		offset = maxScroll
	}
	end := offset + desiredHeight
	if end > total {
		end = total
	}
	return offset, end, desiredHeight
}

func sortedHistory(rows []types.DailyUsage) []types.DailyUsage {
	result := make([]types.DailyUsage, len(rows))
	copy(result, rows)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Date > result[i].Date {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
