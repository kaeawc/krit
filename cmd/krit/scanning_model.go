package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// scanningModel drives the parallel profile scans. It owns all scan
// progress state, emits strictProgressMsg updates while the strict
// profile runs, and sends scansDoneMsg when every profile is done.
type scanningModel struct {
	opts     onboarding.ScanOptions
	target   string
	profiles []string
	cursor   int

	scanStart         time.Time
	profileStart      map[string]time.Time
	profileDurations  map[string]time.Duration
	scanTotalDuration time.Duration

	strictStageIdx   int
	strictStageLabel string
	strictStageTotal int
	strictEvents     chan tea.Msg

	now          time.Time
	scanProgress progress.Model
	scans        map[string]*onboarding.ScanResult
}

// scansDoneMsg is sent when all profiles have been scanned.
type scansDoneMsg struct {
	scans             map[string]*onboarding.ScanResult
	scanTotalDuration time.Duration
	profileDurations  map[string]time.Duration
}

// scanErrorMsg is sent when a scan returns an error.
type scanErrorMsg struct{ err error }

// scanDoneMsg carries the result of a single profile scan.
type scanDoneMsg struct {
	profile string
	result  *onboarding.ScanResult
	err     error
}

// strictProgressMsg fires each time `krit -v` advances a stage during
// the strict scan.
type strictProgressMsg struct {
	index int
	label string
	total int
}

// tickMsg is a periodic heartbeat that refreshes elapsed timers while
// a scan is running.
type tickMsg time.Time

func newScanningModel(opts onboarding.ScanOptions, target string) scanningModel {
	now := time.Now()
	profiles := onboarding.ProfileNames
	return scanningModel{
		opts:             opts,
		target:           target,
		profiles:         profiles,
		cursor:           0,
		scanStart:        now,
		now:              now,
		profileStart:     map[string]time.Time{profiles[0]: now},
		profileDurations: make(map[string]time.Duration),
		strictStageTotal: len(onboarding.StrictStages),
		strictEvents:     make(chan tea.Msg, 16),
		scanProgress:     progress.New(progress.WithDefaultGradient(), progress.WithWidth(30), progress.WithoutPercentage()),
		scans:            make(map[string]*onboarding.ScanResult),
	}
}

func (m scanningModel) Init() tea.Cmd {
	return tea.Batch(m.scanNextCmd(), m.drainStrictEventsCmd(), tickCmd())
}

func (m scanningModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.scanProgress.Width = msg.Width / 3
		return m, nil

	case tickMsg:
		m.now = time.Time(msg)
		return m, tickCmd()

	case strictProgressMsg:
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
			return m, func() tea.Msg { return scanErrorMsg{err: msg.err} }
		}
		m.scans[msg.profile] = msg.result
		now := time.Now()
		m.now = now
		if start, ok := m.profileStart[msg.profile]; ok {
			m.profileDurations[msg.profile] = now.Sub(start)
		}
		if msg.profile == "strict" {
			m.strictStageIdx = m.strictStageTotal
			m.strictStageLabel = "done"
		}
		m.cursor++
		if m.cursor < len(m.profiles) {
			m.profileStart[m.profiles[m.cursor]] = now
			next := m.scanNextCmd()
			if m.profiles[m.cursor] == "strict" {
				return m, tea.Batch(next, m.drainStrictEventsCmd())
			}
			return m, next
		}
		// All scans done — emit scansDoneMsg via Cmd so the root model
		// can update shared state and transition to picker.
		duration := now.Sub(m.scanStart)
		scans := m.scans
		durations := m.profileDurations
		return m, func() tea.Msg {
			return scansDoneMsg{
				scans:             scans,
				scanTotalDuration: duration,
				profileDurations:  durations,
			}
		}
	}
	return m, nil
}

func (m scanningModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("target: %s", m.target)))
	b.WriteString("\n\n")

	total := m.strictStageTotal
	if total <= 0 {
		total = len(onboarding.StrictStages)
	}
	idx := m.strictStageIdx
	if idx > total {
		idx = total
	}
	pct := float64(0)
	if total > 0 {
		pct = float64(idx) / float64(total)
	}
	bar := m.scanProgress.ViewAs(pct)

	label := m.strictStageLabel
	if label == "" {
		label = "starting strict scan..."
	}
	elapsed := durationLabel(m.elapsedSince(m.profileStart["strict"]))
	b.WriteString(fmt.Sprintf("  strict  %s  %d/%d  %s\n", bar, idx, total, elapsed))
	b.WriteString(dimStyle.Render(fmt.Sprintf("          %s", label)))
	b.WriteString("\n\n")

	for i, p := range m.profiles {
		var status string
		switch {
		case i < m.cursor:
			res := m.scans[p]
			findings := 0
			if res != nil {
				findings = res.Total
			}
			status = accentStyle.Render(fmt.Sprintf("done (%d findings, %s)",
				findings, durationLabel(m.profileDurations[p])))
		case i == m.cursor:
			status = fmt.Sprintf("▸ scanning... (%s)",
				durationLabel(m.elapsedSince(m.profileStart[p])))
		default:
			status = dimStyle.Render("pending")
		}
		b.WriteString(fmt.Sprintf("  %-14s %s\n", p, status))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("q to cancel"))
	return b.String()
}

func (m scanningModel) elapsedSince(start time.Time) time.Duration {
	if start.IsZero() {
		return 0
	}
	ref := m.now
	if ref.IsZero() {
		ref = time.Now()
	}
	if ref.Before(start) {
		return 0
	}
	return ref.Sub(start)
}

// scanNextCmd kicks off the next profile scan. The strict profile uses
// a progress-aware helper that tails krit's verbose stderr.
func (m scanningModel) scanNextCmd() tea.Cmd {
	if m.cursor >= len(m.profiles) {
		return nil
	}
	profile := m.profiles[m.cursor]
	opts := m.opts
	if profile == "strict" {
		ch := m.strictEvents
		return func() tea.Msg {
			go func() {
				res, err := onboarding.ScanProfileWithProgress(
					context.Background(),
					opts,
					profile,
					func(ev onboarding.ProgressEvent) {
						ch <- strictProgressMsg{
							index: ev.StageIndex,
							label: ev.StageLabel,
							total: ev.TotalStages,
						}
					},
				)
				ch <- scanDoneMsg{profile: profile, result: res, err: err}
			}()
			return nil
		}
	}
	return func() tea.Msg {
		res, err := onboarding.ScanProfile(context.Background(), opts, profile)
		return scanDoneMsg{profile: profile, result: res, err: err}
	}
}

// drainStrictEventsCmd blocks on the strict event channel and returns
// the next message. Re-issued after every strict progress/done event.
func (m scanningModel) drainStrictEventsCmd() tea.Cmd {
	ch := m.strictEvents
	return func() tea.Msg {
		return <-ch
	}
}

// tickCmd schedules a tickMsg ~100ms from now.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// durationLabel formats a duration as Xms (under 1s), X.Xs (under
// 1m), or XmY.Ys (1m+). Zero renders as a dash.
func durationLabel(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := d.Seconds() - float64(mins)*60
	return fmt.Sprintf("%dm%.1fs", mins, secs)
}
