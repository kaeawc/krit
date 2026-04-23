package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"

	"github.com/kaeawc/krit/internal/onboarding"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

// ruleExplorerItem is a single row in the explorer left pane.
type ruleExplorerItem struct {
	name    string
	ruleset string
	count   int
	ruleRef *v2rules.Rule
}

// explorerFixtureLoadedMsg carries a lazily-loaded fixture pair for
// the explorer pane.
type explorerFixtureLoadedMsg struct {
	ruleName string
	pair     fixturePair
}

// explorerModel drives the rule browser phase. The user can toggle
// rules on/off and see their fixtures in a split pane. When the user
// commits with enter, it emits explorerDoneMsg with the overrides.
type explorerModel struct {
	selected string
	scans    map[string]*onboarding.ScanResult
	repoRoot string

	ruleItems          []ruleExplorerItem
	ruleActive         map[string]bool
	explorerCursor     int
	explorerOffset     int
	explorerFixtureCache map[string]fixturePair
	liveTotal          int

	width  int
	height int
}

func newExplorerModel(
	selected string,
	scans map[string]*onboarding.ScanResult,
	repoRoot string,
	width, height int,
) explorerModel {
	m := explorerModel{
		selected:             selected,
		scans:                scans,
		repoRoot:             repoRoot,
		ruleActive:           make(map[string]bool),
		explorerFixtureCache: make(map[string]fixturePair),
		width:                width,
		height:               height,
	}
	m.buildRuleItems()
	return m
}

// buildRuleItems populates ruleItems from the global rule registry.
func (m *explorerModel) buildRuleItems() {
	scan := m.scans[m.selected]
	seen := make(map[string]bool, len(v2rules.Registry))
	for _, r := range v2rules.Registry {
		name := r.ID
		if seen[name] {
			continue
		}
		seen[name] = true
		count := 0
		if scan != nil {
			count = scan.ByRule[name]
		}
		m.ruleItems = append(m.ruleItems, ruleExplorerItem{
			name:    name,
			ruleset: r.Category,
			count:   count,
			ruleRef: r,
		})
		if _, ok := m.ruleActive[name]; !ok {
			m.ruleActive[name] = rules.IsDefaultActive(name)
		}
	}
	sort.Slice(m.ruleItems, func(i, j int) bool {
		if m.ruleItems[i].ruleset != m.ruleItems[j].ruleset {
			return m.ruleItems[i].ruleset < m.ruleItems[j].ruleset
		}
		return m.ruleItems[i].name < m.ruleItems[j].name
	})
	if scan != nil {
		m.liveTotal = scan.Total
	}
}

func (m explorerModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case explorerFixtureLoadedMsg:
		if m.explorerFixtureCache == nil {
			m.explorerFixtureCache = make(map[string]fixturePair)
		}
		m.explorerFixtureCache[msg.ruleName] = msg.pair
		return m, nil

	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m explorerModel) updateKey(msg tea.KeyMsg) (phaseModel, tea.Cmd) {
	if len(m.ruleItems) == 0 {
		return m, m.commitCmd()
	}

	visible := m.height - 7
	if visible < 5 {
		visible = 5
	}
	if visible > 30 {
		visible = 30
	}

	switch msg.String() {
	case "up", "k":
		if m.explorerCursor > 0 {
			m.explorerCursor--
			if m.explorerCursor < m.explorerOffset {
				m.explorerOffset = m.explorerCursor
			}
		}
	case "down", "j":
		if m.explorerCursor < len(m.ruleItems)-1 {
			m.explorerCursor++
			if m.explorerCursor >= m.explorerOffset+visible {
				m.explorerOffset = m.explorerCursor - visible + 1
			}
		}
	case "pgup":
		m.explorerCursor -= visible
		if m.explorerCursor < 0 {
			m.explorerCursor = 0
		}
		m.explorerOffset = m.explorerCursor
	case "pgdown":
		m.explorerCursor += visible
		if m.explorerCursor >= len(m.ruleItems) {
			m.explorerCursor = len(m.ruleItems) - 1
		}
		if m.explorerCursor >= m.explorerOffset+visible {
			m.explorerOffset = m.explorerCursor - visible + 1
		}
	case " ", "space":
		item := m.ruleItems[m.explorerCursor]
		prev := m.ruleActive[item.name]
		m.ruleActive[item.name] = !prev
		if prev {
			m.liveTotal -= item.count
		} else {
			m.liveTotal += item.count
		}
	case "enter":
		return m, m.commitCmd()
	}
	return m, m.maybeLoadFixtureCmd()
}

func (m explorerModel) commitCmd() tea.Cmd {
	var overrides []onboarding.Override
	for _, item := range m.ruleItems {
		def := rules.IsDefaultActive(item.name)
		if m.ruleActive[item.name] == def {
			continue
		}
		overrides = append(overrides, onboarding.Override{
			Ruleset: item.ruleset,
			Rule:    item.name,
			Active:  m.ruleActive[item.name],
		})
	}
	return func() tea.Msg {
		return explorerDoneMsg{overrides: overrides}
	}
}

func (m *explorerModel) maybeLoadFixtureCmd() tea.Cmd {
	if m.explorerCursor >= len(m.ruleItems) {
		return nil
	}
	item := m.ruleItems[m.explorerCursor]
	if m.explorerFixtureCache == nil {
		m.explorerFixtureCache = make(map[string]fixturePair)
	}
	if _, loaded := m.explorerFixtureCache[item.name]; loaded {
		return nil
	}
	m.explorerFixtureCache[item.name] = fixturePair{} // sentinel
	return m.loadFixtureCmd(item.name, item.ruleset)
}

func (m explorerModel) loadFixtureCmd(ruleName, ruleset string) tea.Cmd {
	repoRoot := m.repoRoot
	return func() tea.Msg {
		pair := fixturePair{}
		posPath := filepath.Join(repoRoot, "tests", "fixtures", "positive", ruleset, ruleName+".kt")
		if data, err := os.ReadFile(posPath); err == nil {
			pair.positive = string(data)
		} else if !os.IsNotExist(err) {
			pair.posErr = err.Error()
		}
		negPath := filepath.Join(repoRoot, "tests", "fixtures", "negative", ruleset, ruleName+".kt")
		if data, err := os.ReadFile(negPath); err == nil {
			pair.negative = string(data)
		} else if !os.IsNotExist(err) {
			pair.negErr = err.Error()
		}
		fixPath := filepath.Join(repoRoot, "tests", "fixtures", "fixable", "per-rule", ruleName+".kt")
		expPath := fixPath + ".expected"
		if before, err := os.ReadFile(fixPath); err == nil {
			if after, err := os.ReadFile(expPath); err == nil {
				pair.fixBefore = string(before)
				pair.fixAfter = string(after)
			}
		}
		return explorerFixtureLoadedMsg{ruleName: ruleName, pair: pair}
	}
}

func (m explorerModel) View() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("krit init — rule explorer"))
	header.WriteString("\n")
	header.WriteString(dimStyle.Render(fmt.Sprintf("profile: %s   rules: %d   live finding count: %d",
		m.selected, len(m.ruleItems), m.liveTotal)))
	header.WriteString("\n\n")

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

	var right strings.Builder
	if m.explorerCursor < len(m.ruleItems) {
		item := m.ruleItems[m.explorerCursor]
		state := "off"
		if m.ruleActive[item.name] {
			state = "on"
		}
		right.WriteString(selectedStyle.Render(item.name) + "\n")
		right.WriteString(dimStyle.Render("ruleset: " + item.ruleset + "   state: " + state))
		right.WriteString("\n\n")
		if item.ruleRef != nil {
			if desc := item.ruleRef.Description; desc != "" {
				right.WriteString(wordwrap.String(desc, rightWidth-6) + "\n\n")
			}
		}
		right.WriteString(fmt.Sprintf("%d finding(s) in this scan", item.count))
		scan := m.scans[m.selected]
		if scan != nil && len(scan.Findings[item.name]) > 0 {
			right.WriteString("\n")
			for _, f := range scan.Findings[item.name] {
				rel := f.File
				if m.repoRoot != "" {
					if r, err := filepath.Rel(m.repoRoot, f.File); err == nil {
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

		pair, hasPair := m.explorerFixtureCache[item.name]
		hasFixture := hasPair && (pair.positive != "" || pair.negative != "" ||
			pair.posErr != "" || pair.negErr != "")
		if hasFixture {
			right.WriteString(m.renderFixturePane(pair, m.ruleActive[item.name], rightWidth) + "\n")
		} else if !hasPair {
			right.WriteString(dimStyle.Render("(loading fixture…)") + "\n")
		}
		right.WriteString(dimStyle.Render("space toggle · enter commit · q quit"))
	}

	leftPane := lipgloss.NewStyle().Width(leftWidth).Render(left.String())
	rightPane := boxStyle.Width(rightWidth - 2).Render(right.String())
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, "  ", rightPane)
	return header.String() + panes + "\n\n" + dimStyle.Render("↑/↓ nav · pgup/pgdn · space toggle · enter commit · q quit")
}

func (m explorerModel) renderFixturePane(pair fixturePair, active bool, width int) string {
	if !active {
		return boxStyle.Width(width - 2).Render(
			dimStyle.Render("rule disabled — these patterns will not be flagged"))
	}
	body := renderFixtureContent(pair, width-4)
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
