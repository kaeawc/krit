package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"

	"github.com/kaeawc/krit/internal/onboarding"
)

// questionnaireModel drives the per-question yes/no flow. It owns the
// visible question list, the current cursor, the accumulated answers,
// and the live finding count. When all questions are answered it emits
// questionnaireDoneMsg so the root model can transition to thresholds.
type questionnaireModel struct {
	registry *onboarding.Registry
	selected string
	scans    map[string]*onboarding.ScanResult
	acceptAll bool
	repoRoot  string

	visibleQs      []int
	qIdx           int
	qCursor        int
	answers        []onboarding.Answer
	cascaded       map[string]bool
	liveTotal      int
	fixtureCache   map[string]fixturePair
	fixtureViewport viewport.Model

	width  int
	height int
}

// fixturesLoadedMsg carries all fixture data loaded asynchronously.
type fixturesLoadedMsg struct {
	cache map[string]fixturePair
}

func newQuestionnaireModel(
	registry *onboarding.Registry,
	selected string,
	scans map[string]*onboarding.ScanResult,
	acceptAll bool,
	repoRoot string,
	width, height int,
) questionnaireModel {
	m := questionnaireModel{
		registry:    registry,
		selected:    selected,
		scans:       scans,
		acceptAll:   acceptAll,
		repoRoot:    repoRoot,
		cascaded:    make(map[string]bool),
		fixtureCache: make(map[string]fixturePair),
		width:       width,
		height:      height,
	}
	m.buildVisibleQs()
	if res := scans[selected]; res != nil {
		m.liveTotal = res.Total
	}
	if len(m.visibleQs) > 0 {
		first := &registry.Questions[m.visibleQs[0]]
		if first.Defaults[selected] {
			m.qCursor = 0
		} else {
			m.qCursor = 1
		}
	}
	m.syncFixtureViewport()
	return m
}

func (m questionnaireModel) Init() tea.Cmd {
	return loadFixturesCmd(m.registry.Questions, m.repoRoot)
}

func (m questionnaireModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncFixtureViewport()
		return m, nil

	case fixturesLoadedMsg:
		for k, v := range msg.cache {
			m.fixtureCache[k] = v
		}
		m.syncFixtureViewport()
		return m, nil

	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m questionnaireModel) updateKey(msg tea.KeyMsg) (phaseModel, tea.Cmd) {
	if m.qIdx >= len(m.visibleQs) {
		return m, func() tea.Msg {
			return questionnaireDoneMsg{answers: m.answers, liveTotal: m.liveTotal}
		}
	}
	q := &m.registry.Questions[m.visibleQs[m.qIdx]]

	switch msg.String() {
	case "left", "h":
		m.qCursor = 0
		m.syncFixtureViewport()
	case "right", "l":
		m.qCursor = 1
		m.syncFixtureViewport()
	case "y", "Y":
		m.qCursor = 0
		m.syncFixtureViewport()
	case "n", "N":
		m.qCursor = 1
		m.syncFixtureViewport()
	case "up", "k", "down", "j", "pgup", "pgdown":
		var cmd tea.Cmd
		m.fixtureViewport, cmd = m.fixtureViewport.Update(msg)
		return m, cmd
	case "enter", " ":
		value := m.qCursor == 0
		m = m.applyAnswer(q, value)
		m.qIdx++
		if m.qIdx < len(m.visibleQs) {
			next := &m.registry.Questions[m.visibleQs[m.qIdx]]
			if next.Defaults[m.selected] {
				m.qCursor = 0
			} else {
				m.qCursor = 1
			}
		} else {
			return m, func() tea.Msg {
				return questionnaireDoneMsg{answers: m.answers, liveTotal: m.liveTotal}
			}
		}
		m.fixtureViewport.GotoTop()
		m.syncFixtureViewport()
	}
	return m, nil
}

func (m questionnaireModel) View() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("krit init — questionnaire"))
	header.WriteString("\n")
	header.WriteString(dimStyle.Render(fmt.Sprintf("profile: %s   live finding count: %d", m.selected, m.liveTotal)))
	header.WriteString("\n\n")

	if m.qIdx >= len(m.visibleQs) {
		return header.String() + "done.\n"
	}
	q := &m.registry.Questions[m.visibleQs[m.qIdx]]

	total := m.width
	if total <= 0 {
		total = 120
	}
	const minSplit = 100
	stacked := total < minSplit
	leftWidth := total - 2
	rightWidth := 0
	if !stacked {
		rightWidth = total / 2
		if rightWidth < 40 {
			rightWidth = 40
		}
		leftWidth = total - rightWidth - 4
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

func (m questionnaireModel) renderQuestionPane(q *onboarding.Question, width int) string {
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

func (m questionnaireModel) renderFixturePane(width int) string {
	return boxStyle.Width(width - 2).Render(m.fixtureViewport.View())
}

// syncFixtureViewport updates the viewport content and dimensions
// based on the current question and cursor state.
func (m *questionnaireModel) syncFixtureViewport() {
	width := m.width / 2
	if width < 40 {
		width = 40
	}
	headerHeight := 3
	borderHeight := 2
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

// applyAnswer records an answer and adjusts liveTotal. It is a pure
// value receiver that returns the updated model.
func (m questionnaireModel) applyAnswer(q *onboarding.Question, value bool) questionnaireModel {
	m.answers = append(m.answers, onboarding.Answer{QuestionID: q.ID, Value: value})

	active := value
	if q.Invert() {
		active = !active
	}
	if !active {
		if res := m.scans[m.selected]; res != nil {
			for _, rule := range q.Rules {
				m.liveTotal -= res.ByRule[rule]
			}
		}
	}

	for _, childID := range q.CascadesTo {
		for i := range m.registry.Questions {
			child := &m.registry.Questions[i]
			if child.ID != childID {
				continue
			}
			bucket := "relaxed"
			if value {
				bucket = "strict"
			}
			derived := child.Defaults[bucket]
			m.answers = append(m.answers, onboarding.Answer{
				QuestionID: child.ID, Value: derived, Cascaded: true, Parent: q.ID,
			})
			m.cascaded[child.ID] = true

			childActive := derived
			if child.Invert() {
				childActive = !childActive
			}
			if !childActive {
				if res := m.scans[m.selected]; res != nil {
					for _, rule := range child.Rules {
						m.liveTotal -= res.ByRule[rule]
					}
				}
			}
		}
	}
	return m
}

// buildVisibleQs computes the list of non-cascaded question indices.
func (m *questionnaireModel) buildVisibleQs() {
	m.visibleQs = m.visibleQs[:0]
	for i := range m.registry.Questions {
		q := &m.registry.Questions[i]
		if q.CascadeFrom != nil {
			continue
		}
		m.visibleQs = append(m.visibleQs, i)
	}
}

// autoAcceptAll applies every question's per-profile default and
// returns the updated model. Called when acceptAll is set.
func (m questionnaireModel) autoAcceptAll() questionnaireModel {
	for _, qi := range m.visibleQs {
		q := &m.registry.Questions[qi]
		def := q.Defaults[m.selected]
		m = m.applyAnswer(q, def)
	}
	return m
}

// ---------- fixture loading -------------------------------------------------

// loadFixturesCmd reads all fixture files from disk in a goroutine and
// delivers the result as a fixturesLoadedMsg.
func loadFixturesCmd(questions []onboarding.Question, repoRoot string) tea.Cmd {
	return func() tea.Msg {
		cache := make(map[string]fixturePair, len(questions))
		for i := range questions {
			q := &questions[i]
			pair := fixturePair{}
			if q.PositiveFixture != nil {
				data, err := os.ReadFile(filepath.Join(repoRoot, *q.PositiveFixture))
				if err != nil {
					pair.posErr = err.Error()
				} else {
					pair.positive = string(data)
				}
			}
			if q.NegativeFixture != nil {
				data, err := os.ReadFile(filepath.Join(repoRoot, *q.NegativeFixture))
				if err != nil {
					pair.negErr = err.Error()
				} else {
					pair.negative = string(data)
				}
			}
			for _, ruleName := range q.Rules {
				fixPath := filepath.Join(repoRoot, "tests", "fixtures", "fixable", "per-rule", ruleName+".kt")
				expPath := fixPath + ".expected"
				before, errB := os.ReadFile(fixPath)
				after, errA := os.ReadFile(expPath)
				if errB == nil && errA == nil {
					pair.fixBefore = string(before)
					pair.fixAfter = string(after)
					break
				}
			}
			cache[q.ID] = pair
		}
		return fixturesLoadedMsg{cache: cache}
	}
}
