package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// ---------- writingPhaseModel -----------------------------------------------

// writingPhaseModel is a passive loading screen shown while writeConfigCmd runs.
type writingPhaseModel struct{}

func (writingPhaseModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	return writingPhaseModel{}, nil
}
func (writingPhaseModel) View() string {
	return titleStyle.Render("krit init") + "\n\n" + dimStyle.Render("writing config...")
}

// ---------- autofixRunningPhaseModel ----------------------------------------

type autofixRunningPhaseModel struct{}

func (autofixRunningPhaseModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	return autofixRunningPhaseModel{}, nil
}
func (autofixRunningPhaseModel) View() string {
	return titleStyle.Render("krit init — autofix") + "\n\n" +
		dimStyle.Render("applying safe autofixes...\n(re-scanning before and after to measure the delta)")
}

// ---------- baselineRunningPhaseModel ---------------------------------------

type baselineRunningPhaseModel struct{}

func (baselineRunningPhaseModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	return baselineRunningPhaseModel{}, nil
}
func (baselineRunningPhaseModel) View() string {
	return titleStyle.Render("krit init — baseline") + "\n\n" +
		dimStyle.Render("writing .krit/baseline.xml...")
}

// ---------- autofixConfirmPhaseModel ----------------------------------------

// autofixConfirmPhaseModel wraps confirmModel for the autofix step.
type autofixConfirmPhaseModel struct {
	m confirmModel
}

func newAutofixConfirmPhase() autofixConfirmPhaseModel {
	return autofixConfirmPhaseModel{
		m: newConfirmModel(
			"krit init — autofix",
			"Apply safe autofixes now?\n"+
				dimStyle.Render("Runs krit --fix at the default idiomatic level against the target.\nThis mutates files in place; consider committing first."),
			true,
		),
	}
}

func (p autofixConfirmPhaseModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	updated, answered, value := p.m.Update(km)
	p.m = updated
	if answered {
		return p, func() tea.Msg { return autofixAnsweredMsg{value: value} }
	}
	return p, nil
}

func (p autofixConfirmPhaseModel) View() string {
	return p.m.View(0)
}

// ---------- baselineConfirmPhaseModel ---------------------------------------

// baselineConfirmPhaseModel wraps confirmModel for the baseline step. It also
// holds the autofix result summary shown above the baseline prompt.
type baselineConfirmPhaseModel struct {
	m              confirmModel
	fixedCount     int
	postfixTotal   int
	topFixedRules  []onboarding.RuleCount
	autofixSkipped bool
}

func newBaselineConfirmPhase(fixedCount, postfixTotal int, topFixedRules []onboarding.RuleCount, autofixSkipped bool) baselineConfirmPhaseModel {
	return baselineConfirmPhaseModel{
		m: newConfirmModel(
			"krit init — baseline",
			"Write a baseline to suppress remaining findings?\n"+
				dimStyle.Render("Writes .krit/baseline.xml so only new findings are flagged going forward."),
			true,
		),
		fixedCount:     fixedCount,
		postfixTotal:   postfixTotal,
		topFixedRules:  topFixedRules,
		autofixSkipped: autofixSkipped,
	}
}

func (p baselineConfirmPhaseModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	updated, answered, value := p.m.Update(km)
	p.m = updated
	if answered {
		return p, func() tea.Msg { return baselineAnsweredMsg{value: value} }
	}
	return p, nil
}

func (p baselineConfirmPhaseModel) View() string {
	if p.autofixSkipped {
		return p.m.View(0)
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(p.m.title))
	b.WriteString("\n\n")
	b.WriteString(accentStyle.Render(fmt.Sprintf("✓ fixed %d findings", p.fixedCount)))
	b.WriteString(fmt.Sprintf("  (remaining: %d)\n", p.postfixTotal))
	if len(p.topFixedRules) > 0 {
		b.WriteString("\n")
		for _, t := range p.topFixedRules {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  -%d %s", t.Count, t.Name)) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(p.m.description)
	b.WriteString("\n\n")
	yes, no := confirmButtons(p.m.cursor)
	b.WriteString("  " + yes + "   " + no + "\n\n")
	b.WriteString(dimStyle.Render("←/→ y/n pick · enter confirm · q quit"))
	return b.String()
}

// ---------- donePhaseModel --------------------------------------------------

// donePhaseModel shows the final summary after all steps complete.
type donePhaseModel struct {
	configPath      string
	selected        string
	liveTotal       int
	target          string
	autofixSkipped  bool
	fixedCount      int
	postfixTotal    int
	topFixedRules   []onboarding.RuleCount
	baselineSkipped bool
	baselinePath    string
	baselineWritten bool
}

func (p donePhaseModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "enter" || km.String() == "esc" {
			return p, tea.Quit
		}
	}
	return p, nil
}

func (p donePhaseModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init — done"))
	b.WriteString("\n\n")
	b.WriteString(accentStyle.Render("✓ config written"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", p.configPath))
	b.WriteString(fmt.Sprintf("  based on: %s profile\n", p.selected))
	b.WriteString(fmt.Sprintf("  live finding count: %d\n", p.liveTotal))
	b.WriteString("\n")

	if p.autofixSkipped {
		b.WriteString(dimStyle.Render("autofix: skipped"))
		b.WriteString("\n")
	} else if p.fixedCount > 0 || p.postfixTotal > 0 {
		b.WriteString(accentStyle.Render(fmt.Sprintf("✓ autofix: %d fixed, %d remaining", p.fixedCount, p.postfixTotal)))
		b.WriteString("\n")
	}

	if p.baselineSkipped {
		b.WriteString(dimStyle.Render("baseline: skipped"))
		b.WriteString("\n")
	} else if p.baselineWritten {
		b.WriteString(accentStyle.Render("✓ baseline written"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s\n", p.baselinePath))
	}

	b.WriteString("\n")
	b.WriteString("Next steps:\n")
	rel := func(path string) string {
		if r, err := filepath.Rel(p.target, path); err == nil {
			return r
		}
		return path
	}
	parts := []string{rel(p.configPath)}
	if p.baselineWritten {
		parts = append(parts, rel(p.baselinePath))
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  git add %s", strings.Join(parts, " "))))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  git commit -m 'chore: configure krit'"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("press enter to exit"))
	return b.String()
}
