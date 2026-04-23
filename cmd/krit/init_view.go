package main

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------- styles ----------------------------------------------------------

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

// ---------- View ------------------------------------------------------------

func (m initModel) View() string {
	if m.err != nil {
		return errorStyle.Render("error: ") + m.err.Error() + "\n"
	}
	return m.phase.View()
}

// ---------- shared rendering helpers ----------------------------------------

// renderTable produces a plain-text comparison table with the given
// header row, data rows, and an optional highlighted row index.
func renderTable(headers []string, rows [][]string, highlight int) string {
	cols := len(headers)
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if w := lipgloss.Width(c); w > widths[i] {
				widths[i] = w
			}
		}
	}
	const maxTopCol = 55
	if widths[cols-1] > maxTopCol {
		widths[cols-1] = maxTopCol
	}

	var b strings.Builder
	writeRow := func(cells []string, style lipgloss.Style) {
		parts := make([]string, cols)
		for i, c := range cells {
			if i == cols-1 && lipgloss.Width(c) > widths[i] {
				c = c[:widths[i]-1] + "…"
			}
			parts[i] = padRight(c, widths[i])
		}
		line := "  " + strings.Join(parts, "  ")
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	writeRow(headers, titleStyle)
	sepCells := make([]string, cols)
	for i, w := range widths {
		sepCells[i] = strings.Repeat("─", w)
	}
	b.WriteString(dimStyle.Render("  " + strings.Join(sepCells, "  ")))
	b.WriteString("\n")
	for i, r := range rows {
		style := lipgloss.NewStyle()
		if i == highlight {
			style = selectedStyle
			copyRow := make([]string, len(r))
			copy(copyRow, r)
			copyRow[0] = "▸ " + copyRow[0]
			writeRow(copyRow, style)
			continue
		}
		writeRow(r, style)
	}
	return b.String()
}

func padRight(s string, width int) string {
	diff := width - lipgloss.Width(s)
	if diff <= 0 {
		return s
	}
	return s + strings.Repeat(" ", diff)
}

// Ensure sort is referenced to keep the import stable.
var _ = sort.Strings
