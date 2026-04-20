package main

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
)

// ---------- Update ----------

func (m initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.scanProgress.Width = msg.Width / 3
		if m.phase == phaseQuestionnaire {
			m.syncFixtureViewport()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		switch m.phase {
		case phasePicker:
			return m.updatePicker(msg)
		case phaseQuestionnaire:
			return m.updateQuestionnaire(msg)
		case phaseThresholds:
			return m.updateThresholds(msg)
		case phaseExplorer:
			return m.updateExplorer(msg)
		case phaseAutofixConfirm:
			return m.updateAutofixConfirm(msg)
		case phaseBaselineConfirm:
			return m.updateBaselineConfirm(msg)
		case phaseDone:
			if msg.String() == "enter" || msg.String() == "esc" {
				return m, tea.Quit
			}
		}

	case tickMsg:
		m.now = time.Time(msg)
		if m.phase == phaseScanning {
			return m, tickCmd()
		}
		return m, nil

	case strictProgressMsg:
		// Record stage progress and keep draining the channel.
		if msg.index > m.strictStageIdx {
			m.strictStageIdx = msg.index
		}
		m.strictStageLabel = msg.label
		if msg.total > 0 {
			m.strictStageTotal = msg.total
		}
		m.now = time.Now()
		return m, m.drainStrictEventsCmd()

	case scanDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.scans[msg.profile] = msg.result
		now := time.Now()
		m.now = now
		if start, ok := m.profileStart[msg.profile]; ok {
			m.profileDurations[msg.profile] = now.Sub(start)
		}
		// When strict finishes, snap the progress bar to 100%.
		if msg.profile == "strict" {
			m.strictStageIdx = m.strictStageTotal
			m.strictStageLabel = "done"
		}
		m.scanCursor++
		if m.scanCursor < len(m.profiles) {
			m.profileStart[m.profiles[m.scanCursor]] = now
			// If the profile we just finished was strict, we must
			// keep draining the channel (the goroutine has exited,
			// but any unread progress events would be stuck) — but
			// the goroutine already sent scanDoneMsg last, so the
			// channel is drained. Only re-arm the drain cmd if the
			// next profile is also strict, which it won't be.
			next := m.scanNextCmd()
			if m.profiles[m.scanCursor] == "strict" {
				return m, tea.Batch(next, m.drainStrictEventsCmd())
			}
			return m, next
		}
		// All scans done — record total duration.
		m.scanTotalDuration = now.Sub(m.scanStart)
		if m.presetProfile != "" {
			m.selected = m.presetProfile
			m.startQuestionnaire()
			if len(m.fixtureCache) == 0 {
				return m, loadFixturesCmd(m.registry.Questions, m.opts.RepoRoot)
			}
			return m, nil
		}
		m.phase = phasePicker
		return m, nil

	case writeDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.configPath = msg.path
		// Gate into autofix confirm; in --yes mode, auto-accept and run.
		if m.acceptAll {
			m.phase = phaseAutofixRunning
			return m, m.autofixCmd()
		}
		m.phase = phaseAutofixConfirm
		m.autofixConfirm = newConfirmModel(
			"krit init — autofix",
			"Apply safe autofixes now?\n"+
				dimStyle.Render("Runs krit --fix at the default idiomatic level against the target.\nThis mutates files in place; consider committing first."),
			true,
		)
		return m, nil

	case autofixDoneMsg:
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
		// In --yes mode, auto-run baseline as well.
		if m.acceptAll {
			m.phase = phaseBaselineRunning
			return m, m.baselineCmd()
		}
		m.phase = phaseBaselineConfirm
		m.baselineConfirm = newConfirmModel(
			"krit init — baseline",
			"Write a baseline to suppress remaining findings?\n"+
				dimStyle.Render("Writes .krit/baseline.xml so only new findings are flagged going forward."),
			true,
		)
		return m, nil

	case baselineDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.baselinePath = msg.path
		m.baselineWritten = true
		m.phase = phaseDone
		// In --yes mode, quit immediately after the done view renders.
		if m.acceptAll {
			return m, tea.Quit
		}
		return m, nil
	case explorerFixtureLoadedMsg:
		if m.explorerFixtureCache == nil {
			m.explorerFixtureCache = make(map[string]fixturePair)
		}
		m.explorerFixtureCache[msg.ruleName] = msg.pair
		return m, nil
	case profileSelectedMsg:
		m.selected = msg.profile
		if msg.browse {
			m.startExplorer()
		} else {
			m.startQuestionnaire()
			if len(m.fixtureCache) == 0 {
				return m, loadFixturesCmd(m.registry.Questions, m.opts.RepoRoot)
			}
		}
		return m, nil

	case fixturesLoadedMsg:
		for k, v := range msg.cache {
			m.fixtureCache[k] = v
		}
		m.syncFixtureViewport()
		return m, nil
	}
	return m, nil
}

func (m initModel) updateAutofixConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, answered, value := m.autofixConfirm.Update(msg)
	m.autofixConfirm = updated
	if answered {
		if value {
			m.phase = phaseAutofixRunning
			return m, m.autofixCmd()
		}
		// Skip autofix: pre/post totals stay zero, jump straight to baseline.
		m.autofixSkipped = true
		m.phase = phaseBaselineConfirm
		m.baselineConfirm = newConfirmModel(
			"krit init — baseline",
			"Write a baseline to suppress remaining findings?\n"+
				dimStyle.Render("Writes .krit/baseline.xml so only new findings are flagged going forward."),
			true,
		)
	}
	return m, nil
}

func (m initModel) updateBaselineConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	updated, answered, value := m.baselineConfirm.Update(msg)
	m.baselineConfirm = updated
	if answered {
		if value {
			m.phase = phaseBaselineRunning
			return m, m.baselineCmd()
		}
		m.baselineSkipped = true
		m.phase = phaseDone
	}
	return m, nil
}

func (m initModel) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

// startExplorer populates the rule list from the global rule
// registry, seeds ruleActive from the current defaults (accounting
// for krit.Registry's DefaultInactive map), and enters phaseExplorer.
func (m *initModel) startExplorer() {
	m.ruleItems = m.ruleItems[:0]
	if m.ruleActive == nil {
		m.ruleActive = make(map[string]bool)
	}

	// Build ruleItems from the rule registry plus scan counts from
	// the selected profile. Rules with no scan count are still
	// included so the user can toggle them on/off. Dedupe by name:
	// at least one rule (AppCompatResource) is registered twice in
	// krit's registry — once for usability and once for resource
	// scanning — and the user-facing explorer should show it
	// once, not twice.
	scan := m.scans[m.selected]
	seen := make(map[string]bool, len(v2rules.Registry))
	for _, r := range v2rules.Registry {
		name := r.ID
		if seen[name] {
			continue
		}
		seen[name] = true
		ruleset := r.Category
		count := 0
		if scan != nil {
			count = scan.ByRule[name]
		}
		m.ruleItems = append(m.ruleItems, ruleExplorerItem{
			name:    name,
			ruleset: ruleset,
			count:   count,
			ruleRef: r,
		})
		if _, ok := m.ruleActive[name]; !ok {
			// Seed from krit's DefaultInactive map (rules not in the
			// map are active by default).
			m.ruleActive[name] = rules.IsDefaultActive(name)
		}
	}
	sort.Slice(m.ruleItems, func(i, j int) bool {
		if m.ruleItems[i].ruleset != m.ruleItems[j].ruleset {
			return m.ruleItems[i].ruleset < m.ruleItems[j].ruleset
		}
		return m.ruleItems[i].name < m.ruleItems[j].name
	})

	// Seed live total with the selected profile's total.
	if scan != nil {
		m.liveTotal = scan.Total
	}
	m.explorerCursor = 0
	m.explorerOffset = 0
	m.explorerFixtureCache = make(map[string]fixturePair)
	m.phase = phaseExplorer
}

func (m initModel) updateExplorer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.ruleItems) == 0 {
		m.phase = phaseWriting
		return m, m.writeConfigCmd()
	}

	// Visible rows: total height minus header (3) + bottom hint (2) + counter (2).
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
		// Update live total: toggling OFF subtracts the count,
		// toggling ON adds it back.
		if prev {
			m.liveTotal -= item.count
		} else {
			m.liveTotal += item.count
		}
	case "enter":
		// Commit: build overrides from every rule whose active
		// state differs from its default. Also carry the explorer
		// flow forward so questionnaire answers don't fight us.
		m.explorerUsed = true
		m.answers = m.answers[:0]
		for _, item := range m.ruleItems {
			def := rules.IsDefaultActive(item.name)
			if m.ruleActive[item.name] == def {
				continue
			}
			m.answers = append(m.answers, onboarding.Answer{
				QuestionID: "explorer:" + item.name,
				Value:      m.ruleActive[item.name],
			})
		}
		// We can't use BuildOverrides because explorer rules aren't
		// in the registry. Pre-build the overrides here and stash
		// them into thresholdOverrides's sibling slot by extending
		// the write command.
		m.phase = phaseWriting
		return m, m.writeExplorerCmd()
	}
	return m, m.maybeLoadExplorerFixture()
}

// maybeLoadExplorerFixture returns a tea.Cmd that loads fixtures for
// the currently selected explorer rule, or nil if already cached.
func (m *initModel) maybeLoadExplorerFixture() tea.Cmd {
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
	// Sentinel to prevent duplicate in-flight loads.
	m.explorerFixtureCache[item.name] = fixturePair{}
	return m.loadExplorerFixtureCmd(item.name, item.ruleset)
}

func (m *initModel) startQuestionnaire() {
	// Compute the list of visible questions (those not cascaded from
	// any prior answer). Since we answer in declaration order and the
	// registry orders parents before children, a simple pre-pass does
	// the work: mark children of every top-level (non-cascaded) question
	// as cascaded.
	m.visibleQs = m.visibleQs[:0]
	m.cascaded = make(map[string]bool)
	for i := range m.registry.Questions {
		q := &m.registry.Questions[i]
		if q.CascadeFrom != nil {
			// Children are only visible if their parent is NOT answered.
			// In our model every parent IS answered, so skip children.
			continue
		}
		m.visibleQs = append(m.visibleQs, i)
	}

	// Fixture loading is async — dispatched via loadFixturesCmd when
	// transitioning to the questionnaire. The fixturesLoadedMsg handler
	// populates m.fixtureCache and calls syncFixtureViewport.

	// Seed live total from the selected profile.
	if res := m.scans[m.selected]; res != nil {
		m.liveTotal = res.Total
	}
	m.answers = m.answers[:0]
	m.qIdx = 0
	m.qCursor = 0
	m.phase = phaseQuestionnaire

	// Seed the yes/no cursor to the first question's per-profile default.
	if len(m.visibleQs) > 0 {
		first := &m.registry.Questions[m.visibleQs[0]]
		if first.Defaults[m.selected] {
			m.qCursor = 0
		} else {
			m.qCursor = 1
		}
	}
	m.syncFixtureViewport()

	// If --yes was passed, auto-answer every question with the default.
	if m.acceptAll {
		for _, qi := range m.visibleQs {
			q := &m.registry.Questions[qi]
			def := q.Defaults[m.selected]
			m.applyAnswer(q, def)
		}
		m.phase = phaseWriting
	}
}

func (m *initModel) applyAnswer(q *onboarding.Question, value bool) {
	m.answers = append(m.answers, onboarding.Answer{QuestionID: q.ID, Value: value})

	// Update live total: for this question's rules, if the effective
	// active state differs from the profile's scan-time state, adjust
	// the count.
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

	// Cascade children using the same bucket logic as ResolveAnswers.
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
}

func (m initModel) updateQuestionnaire(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.qIdx >= len(m.visibleQs) {
		// Already done; transition to thresholds.
		m.startThresholds()
		if m.phase == phaseWriting {
			return m, m.writeConfigCmd()
		}
		return m, nil
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
		// Forward scroll keys to the fixture viewport.
		var cmd tea.Cmd
		m.fixtureViewport, cmd = m.fixtureViewport.Update(msg)
		return m, cmd
	case "enter", " ":
		value := m.qCursor == 0
		m.applyAnswer(q, value)
		m.qIdx++
		// Seed the yes/no cursor to the next question's per-profile default.
		if m.qIdx < len(m.visibleQs) {
			next := &m.registry.Questions[m.visibleQs[m.qIdx]]
			if next.Defaults[m.selected] {
				m.qCursor = 0
			} else {
				m.qCursor = 1
			}
		} else {
			m.startThresholds()
			if m.phase == phaseWriting {
				return m, m.writeConfigCmd()
			}
		}
		m.fixtureViewport.GotoTop()
		m.syncFixtureViewport()
	}
	return m, nil
}

