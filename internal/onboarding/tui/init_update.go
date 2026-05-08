package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// Update handles messages for initModel. Phase-specific logic lives in
// each sub-model; the root handles cross-phase transitions and error
// propagation.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		newPhase, cmd := m.phase.Update(msg)
		m.phase = newPhase
		return m, cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "ctrl+c" || km.String() == "q" {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case scanErrorMsg:
		m.err = msg.err
		return m, tea.Quit
	case scansDoneMsg:
		return m.handleScansDone(msg)
	case profileSelectedMsg:
		return m.handleProfileSelected(msg)
	case questionnaireDoneMsg:
		return m.handleQuestionnaireDone(msg)
	case thresholdsDoneMsg:
		m.phase = writingPhaseModel{}
		return m, m.writeConfigCmd(msg.overrides)
	case explorerDoneMsg:
		return m.handleExplorerDone(msg)
	case writeDoneMsg:
		return m.handleWriteDone(msg)
	case autofixAnsweredMsg:
		return m.handleAutofixAnswered(msg)
	case autofixDoneMsg:
		return m.handleAutofixDone(msg)
	case baselineAnsweredMsg:
		return m.handleBaselineAnswered(msg)
	case baselineDoneMsg:
		return m.handleBaselineDone(msg)
	}

	newPhase, cmd := m.phase.Update(msg)
	m.phase = newPhase
	return m, cmd
}

func (m Model) handleScansDone(msg scansDoneMsg) (tea.Model, tea.Cmd) {
	m.scans = msg.scans
	if m.presetProfile != "" {
		m.selected = m.presetProfile
		qm := newQuestionnaireModel(
			m.registry, m.selected, m.scans,
			m.acceptAll, m.opts.RepoRoot,
			m.width, m.height,
		)
		if m.acceptAll {
			qm = qm.autoAcceptAll()
			return m, func() tea.Msg {
				return questionnaireDoneMsg{answers: qm.answers, liveTotal: qm.liveTotal}
			}
		}
		m.phase = qm
		return m, qm.Init()
	}
	m.phase = newPickerModel(
		onboarding.ProfileNames,
		msg.scans,
		msg.scanTotalDuration,
		msg.profileDurations,
		m.width,
	)
	return m, nil
}

func (m Model) handleProfileSelected(msg profileSelectedMsg) (tea.Model, tea.Cmd) {
	m.selected = msg.profile
	if msg.browse {
		m.phase = newExplorerModel(m.selected, m.scans, m.opts.RepoRoot, m.width, m.height)
		return m, nil
	}
	qm := newQuestionnaireModel(
		m.registry, m.selected, m.scans,
		m.acceptAll, m.opts.RepoRoot,
		m.width, m.height,
	)
	m.phase = qm
	return m, qm.Init()
}

func (m Model) handleQuestionnaireDone(msg questionnaireDoneMsg) (tea.Model, tea.Cmd) {
	m.answers = msg.answers
	m.liveTotal = msg.liveTotal
	tm := newThresholdsModel(m.selected, m.opts.RepoRoot, m.acceptAll)
	m.phase = tm
	return m, tm.Init()
}

func (m Model) handleExplorerDone(msg explorerDoneMsg) (tea.Model, tea.Cmd) {
	m.liveTotal = 0
	if scan := m.scans[m.selected]; scan != nil {
		m.liveTotal = scan.Total
	}
	m.phase = writingPhaseModel{}
	return m, m.writeExplorerCmd(msg.overrides)
}

func (m Model) handleWriteDone(msg writeDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	m.configPath = msg.path
	if m.acceptAll {
		m.phase = autofixRunningPhaseModel{}
		return m, m.autofixCmd()
	}
	m.phase = newAutofixConfirmPhase()
	return m, nil
}

func (m Model) handleAutofixAnswered(msg autofixAnsweredMsg) (tea.Model, tea.Cmd) {
	if msg.value {
		m.phase = autofixRunningPhaseModel{}
		return m, m.autofixCmd()
	}
	m.autofixSkipped = true
	m.phase = newBaselineConfirmPhase(0, 0, nil, true)
	return m, nil
}

func (m Model) handleAutofixDone(msg autofixDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	m.prefixTotal = msg.prefix
	m.postfixTotal = msg.postfix
	m.fixedCount = msg.prefix - msg.postfix
	if m.fixedCount < 0 {
		m.fixedCount = 0
	}
	m.topFixedRules = msg.top
	if m.acceptAll {
		m.phase = baselineRunningPhaseModel{}
		return m, m.baselineCmd()
	}
	m.phase = newBaselineConfirmPhase(m.fixedCount, m.postfixTotal, m.topFixedRules, false)
	return m, nil
}

func (m Model) handleBaselineAnswered(msg baselineAnsweredMsg) (tea.Model, tea.Cmd) {
	if msg.value {
		m.phase = baselineRunningPhaseModel{}
		return m, m.baselineCmd()
	}
	m.baselineSkipped = true
	m.phase = m.newDonePhase()
	return m, nil
}

func (m Model) handleBaselineDone(msg baselineDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	m.baselinePath = msg.path
	m.baselineWritten = true
	m.phase = m.newDonePhase()
	if m.acceptAll {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) newDonePhase() donePhaseModel {
	return donePhaseModel{
		configPath:      m.configPath,
		selected:        m.selected,
		liveTotal:       m.liveTotal,
		target:          m.target,
		autofixSkipped:  m.autofixSkipped,
		fixedCount:      m.fixedCount,
		postfixTotal:    m.postfixTotal,
		topFixedRules:   m.topFixedRules,
		baselineSkipped: m.baselineSkipped,
		baselinePath:    m.baselinePath,
		baselineWritten: m.baselineWritten,
	}
}
