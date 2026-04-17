package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/kaeawc/krit/internal/onboarding"
	"github.com/kaeawc/krit/internal/rules"
)

// ---------- styles ----------

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

// ---------- View ----------

func (m initModel) View() string {
	if m.err != nil {
		return errorStyle.Render("error: ") + m.err.Error() + "\n"
	}
	switch m.phase {
	case phaseScanning:
		return m.viewScanning()
	case phasePicker:
		return m.viewPicker()
	case phaseQuestionnaire:
		return m.viewQuestionnaire()
	case phaseThresholds:
		return m.viewThresholds()
	case phaseExplorer:
		return m.viewExplorer()
	case phaseWriting:
		return titleStyle.Render("krit init") + "\n\n" + dimStyle.Render("writing config...")
	case phaseAutofixConfirm:
		return m.viewAutofixConfirm()
	case phaseAutofixRunning:
		return titleStyle.Render("krit init — autofix") + "\n\n" + dimStyle.Render("applying safe autofixes...\n(re-scanning before and after to measure the delta)")
	case phaseBaselineConfirm:
		return m.viewBaselineConfirm()
	case phaseBaselineRunning:
		return titleStyle.Render("krit init — baseline") + "\n\n" + dimStyle.Render("writing .krit/baseline.xml...")
	case phaseDone:
		return m.viewDone()
	}
	return ""
}

func (m initModel) viewExplorer() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("krit init — rule explorer"))
	header.WriteString("\n")
	header.WriteString(dimStyle.Render(fmt.Sprintf("profile: %s   rules: %d   live finding count: %d",
		m.selected, len(m.ruleItems), m.liveTotal)))
	header.WriteString("\n\n")

	// Pane widths from terminal size.
	total := m.width
	if total <= 0 {
		total = 140
	}
	rightWidth := 50
	if total < 120 {
		rightWidth = 40
	}
	leftWidth := total - rightWidth - 4
	if leftWidth < 50 {
		leftWidth = 50
	}

	// Left pane: scrollable rule list.
	// Visible rows: total height minus header (3) + bottom hint (2) + counter (2).
	visible := m.height - 7
	if visible < 5 {
		visible = 5
	}
	if visible > 30 {
		visible = 30
	}
	start := m.explorerOffset
	end := start + visible
	if end > len(m.ruleItems) {
		end = len(m.ruleItems)
	}

	var left strings.Builder
	for i := start; i < end; i++ {
		item := m.ruleItems[i]
		mark := dimStyle.Render("  ○")
		if m.ruleActive[item.name] {
			mark = accentStyle.Render("  ●")
		}
		row := fmt.Sprintf("%s %-30s %-18s %5d",
			mark,
			truncate(item.name, 30),
			truncate(item.ruleset, 18),
			item.count,
		)
		if i == m.explorerCursor {
			row = selectedStyle.Render("▸" + row[1:])
		} else {
			row = " " + row
		}
		left.WriteString(row + "\n")
	}
	if len(m.ruleItems) > visible {
		left.WriteString("\n" + dimStyle.Render(fmt.Sprintf("  %d / %d", m.explorerCursor+1, len(m.ruleItems))))
	}

	// Right pane: selected rule detail with description, findings, and fixture preview.
	var right strings.Builder
	if m.explorerCursor < len(m.ruleItems) {
		item := m.ruleItems[m.explorerCursor]
		state := "off"
		if m.ruleActive[item.name] {
			state = "on"
		}

		// Header: rule name + ruleset + state.
		right.WriteString(selectedStyle.Render(item.name) + "\n")
		right.WriteString(dimStyle.Render("ruleset: " + item.ruleset + "   state: " + state))
		right.WriteString("\n\n")

		// Description (optional — only for rules implementing DescriptionProvider).
		if item.ruleRef != nil {
			if desc := rules.DescriptionOf(item.ruleRef); desc != "" {
				right.WriteString(wordwrap.String(desc, rightWidth-6) + "\n\n")
			}
		}

		// Sample findings.
		right.WriteString(fmt.Sprintf("%d finding(s) in this scan", item.count))
		scan := m.scans[m.selected]
		if scan != nil && len(scan.Findings[item.name]) > 0 {
			right.WriteString("\n")
			for _, f := range scan.Findings[item.name] {
				rel := f.File
				if m.opts.RepoRoot != "" {
					if r, err := filepath.Rel(m.opts.RepoRoot, f.File); err == nil {
						rel = r
					}
				}
				loc := fmt.Sprintf("%s:%d", rel, f.Line)
				maxLoc := (rightWidth - 6) / 2
				if len(loc) > maxLoc {
					loc = "…" + loc[len(loc)-maxLoc+1:]
				}
				msg := truncate(f.Message, rightWidth-6-len(loc)-2)
				right.WriteString(dimStyle.Render(fmt.Sprintf("  %s  %s", loc, msg)) + "\n")
			}
		}
		right.WriteString("\n")

		// Fixture preview (lazily loaded).
		pair, hasPair := m.explorerFixtureCache[item.name]
		hasFixture := hasPair && (pair.positive != "" || pair.negative != "" ||
			pair.posErr != "" || pair.negErr != "")
		if hasFixture {
			right.WriteString(m.renderExplorerFixturePane(pair, m.ruleActive[item.name], rightWidth) + "\n")
		} else if !hasPair {
			right.WriteString(dimStyle.Render("(loading fixture…)") + "\n")
		}

		// Keyboard hints.
		right.WriteString(dimStyle.Render("space toggle · enter commit · q quit"))
	}

	leftPane := lipgloss.NewStyle().Width(leftWidth).Render(left.String())
	rightPane := boxStyle.Width(rightWidth - 2).Render(right.String())
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, "  ", rightPane)

	return header.String() + panes + "\n\n" + dimStyle.Render("↑/↓ nav · pgup/pgdn · space toggle · enter commit · q quit")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func (m initModel) viewAutofixConfirm() string {
	return m.autofixConfirm.View(m.width)
}

func (m initModel) viewBaselineConfirm() string {
	// Prepend autofix summary before the baseline confirmation prompt.
	if !m.autofixSkipped {
		var b strings.Builder
		b.WriteString(titleStyle.Render(m.baselineConfirm.title))
		b.WriteString("\n\n")
		b.WriteString(accentStyle.Render(fmt.Sprintf("✓ fixed %d findings", m.fixedCount)))
		b.WriteString(fmt.Sprintf("  (remaining: %d)\n", m.postfixTotal))
		if len(m.topFixedRules) > 0 {
			b.WriteString("\n")
			for _, t := range m.topFixedRules {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  -%d %s", t.Count, t.Name)) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(m.baselineConfirm.description)
		b.WriteString("\n\n")
		yes, no := confirmButtons(m.baselineConfirm.cursor)
		b.WriteString("  " + yes + "   " + no + "\n\n")
		b.WriteString(dimStyle.Render("←/→ y/n pick · enter confirm · q quit"))
		return b.String()
	}
	return m.baselineConfirm.View(m.width)
}

// renderScanProgressBar draws a filled/unfilled bar of the given
// width showing idx/total completion.

func (m initModel) viewPicker() string {
	return m.picker.View(m.width, pickerViewData{
		scans:             m.scans,
		scanTotalDuration: m.scanTotalDuration,
		profileDurations:  m.profileDurations,
	})
}

func (m initModel) viewQuestionnaire() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("krit init — questionnaire"))
	header.WriteString("\n")
	header.WriteString(dimStyle.Render(fmt.Sprintf("profile: %s   live finding count: %d", m.selected, m.liveTotal)))
	header.WriteString("\n\n")

	if m.qIdx >= len(m.visibleQs) {
		return header.String() + "done.\n"
	}
	q := &m.registry.Questions[m.visibleQs[m.qIdx]]

	// Compute left/right pane widths based on terminal width. If the
	// terminal is narrow (<100 cols), fall back to stacked layout —
	// left pane only, no preview.
	total := m.width
	if total <= 0 {
		total = 120
	}
	const minSplit = 100
	stacked := total < minSplit
	leftWidth := total - 2
	rightWidth := 0
	if !stacked {
		// Reserve ~45% of width for the preview, minimum 40 columns.
		rightWidth = total / 2
		if rightWidth < 40 {
			rightWidth = 40
		}
		leftWidth = total - rightWidth - 4 // 4 for gap + borders
		if leftWidth < 40 {
			leftWidth = 40
		}
	}

	left := m.renderQuestionPane(q, leftWidth)
	if stacked {
		return header.String() + left
	}
	right := m.renderFixturePane(rightWidth)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	return header.String() + panes
}

func (m initModel) renderQuestionPane(q *onboarding.Question, width int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Question %d of %d\n\n", m.qIdx+1, len(m.visibleQs)))
	b.WriteString(selectedStyle.Render(q.Question))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(wordwrap.String(q.Rationale, width-2)))
	b.WriteString("\n")

	if len(q.Rules) > 0 {
		b.WriteString(dimStyle.Render(wordwrap.String("Controls: "+strings.Join(q.Rules, ", "), width-2)))
		b.WriteString("\n")
	}
	if len(q.CascadesTo) > 0 {
		b.WriteString(dimStyle.Render(wordwrap.String("Cascades to: "+strings.Join(q.CascadesTo, ", "), width-2)))
		b.WriteString("\n")
	}

	// Yes / No buttons.
	var yes, no string
	if m.qCursor == 0 {
		yes = selectedStyle.Render("[ Yes ]")
		no = dimStyle.Render("  No  ")
	} else {
		yes = dimStyle.Render("  Yes  ")
		no = selectedStyle.Render("[ No ]")
	}
	b.WriteString("\n  " + yes + "   " + no + "\n\n")
	b.WriteString(dimStyle.Render("←/→ y/n · ↑/↓ scroll · enter confirm · q quit"))
	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m initModel) renderFixturePane(width int) string {
	return boxStyle.Width(width - 2).Render(m.fixtureViewport.View())
}

// syncFixtureViewport updates the fixture viewport content and
// dimensions based on the current question and cursor state.
// Called from updateQuestionnaire on state changes, not from View.
func (m *initModel) syncFixtureViewport() {
	width := m.width / 2
	if width < 40 {
		width = 40
	}
	// Compute viewport height dynamically: total height minus the
	// header lines (title + subtitle + blank) and the box border (2).
	headerHeight := 3 // title + profile line + blank
	borderHeight := 2 // top + bottom border of boxStyle
	vpHeight := m.height - headerHeight - borderHeight
	if vpHeight < 8 {
		vpHeight = 8
	}

	m.fixtureViewport.Width = width - 4
	m.fixtureViewport.Height = vpHeight

	if m.qIdx >= len(m.visibleQs) {
		m.fixtureViewport.SetContent("")
		return
	}
	q := &m.registry.Questions[m.visibleQs[m.qIdx]]

	pair, ok := m.fixtureCache[q.ID]
	hasPair := ok && (pair.positive != "" || pair.negative != "" ||
		pair.posErr != "" || pair.negErr != "")

	// Parent questions: try first cascade child.
	if !hasPair && len(q.CascadesTo) > 0 {
		for _, childID := range q.CascadesTo {
			if cp, found := m.fixtureCache[childID]; found &&
				(cp.positive != "" || cp.negative != "") {
				pair = cp
				hasPair = true
				break
			}
		}
	}

	if !hasPair {
		m.fixtureViewport.SetContent(dimStyle.Render("(no fixture to preview)"))
		return
	}

	ruleActive := m.qCursor == 0
	if q.Invert() {
		ruleActive = !ruleActive
	}
	if !ruleActive {
		m.fixtureViewport.SetContent(dimStyle.Render("rule disabled — these patterns will not be flagged"))
		return
	}

	body := renderFixtureContent(pair, width-4)
	m.fixtureViewport.SetContent(body)
}

// renderExplorerFixturePane renders fixture content for the explorer
// right pane. When the rule is inactive, shows a dimmed disabled
// message instead.
func (m initModel) renderExplorerFixturePane(pair fixturePair, active bool, width int) string {
	if !active {
		return boxStyle.Width(width - 2).Render(
			dimStyle.Render("rule disabled — these patterns will not be flagged"))
	}

	body := renderFixtureContent(pair, width-4)

	// Explorer has more overhead: header (3) + description (~3) +
	// findings (~4) + hints (1) + border (2) + bottom hint (2).
	overhead := 15
	vpHeight := m.height - overhead
	if vpHeight < 6 {
		vpHeight = 6
	}
	if vpHeight > 20 {
		vpHeight = 20
	}
	vp := viewport.New(width-4, vpHeight)
	vp.SetContent(body)
	return boxStyle.Width(width - 2).Render(vp.View())
}

func (m initModel) viewDone() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init — done"))
	b.WriteString("\n\n")
	b.WriteString(accentStyle.Render("✓ config written"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", m.configPath))
	b.WriteString(fmt.Sprintf("  based on: %s profile\n", m.selected))
	b.WriteString(fmt.Sprintf("  live finding count: %d\n", m.liveTotal))
	b.WriteString("\n")

	if m.autofixSkipped {
		b.WriteString(dimStyle.Render("autofix: skipped"))
		b.WriteString("\n")
	} else if m.fixedCount > 0 || m.prefixTotal > 0 {
		b.WriteString(accentStyle.Render(fmt.Sprintf("✓ autofix: %d fixed, %d remaining", m.fixedCount, m.postfixTotal)))
		b.WriteString("\n")
	}

	if m.baselineSkipped {
		b.WriteString(dimStyle.Render("baseline: skipped"))
		b.WriteString("\n")
	} else if m.baselineWritten {
		b.WriteString(accentStyle.Render("✓ baseline written"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s\n", m.baselinePath))
	}

	b.WriteString("\n")
	b.WriteString("Next steps:\n")
	rel := func(p string) string {
		if r, err := filepath.Rel(m.target, p); err == nil {
			return r
		}
		return p
	}
	parts := []string{rel(m.configPath)}
	if m.baselineWritten {
		parts = append(parts, rel(m.baselinePath))
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  git add %s", strings.Join(parts, " "))))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  git commit -m 'chore: configure krit'"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("press enter to exit"))
	return b.String()
}

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
	// Cap Top rules column to keep lines readable.
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
	// Separator.
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
			// Prepend a marker column visually.
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

// Ensure sort is referenced even if the rest of the file reshuffles.
var _ = sort.Strings
