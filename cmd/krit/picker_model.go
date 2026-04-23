package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// pickerModel encapsulates the profile-picker phase of krit init.
// It holds scan data needed for the comparison table and manages
// cursor navigation.
type pickerModel struct {
	profiles         []string
	cursor           int
	scans            map[string]*onboarding.ScanResult
	scanTotalDuration time.Duration
	profileDurations map[string]time.Duration
	width            int
}

// profileSelectedMsg is emitted when the user commits a profile
// selection. browse=true indicates the user pressed 'b' to enter the
// rule explorer instead of the guided questionnaire.
type profileSelectedMsg struct {
	profile string
	browse  bool
}

func newPickerModel(
	profiles []string,
	scans map[string]*onboarding.ScanResult,
	scanTotalDuration time.Duration,
	profileDurations map[string]time.Duration,
	width int,
) pickerModel {
	return pickerModel{
		profiles:          profiles,
		cursor:            0,
		scans:             scans,
		scanTotalDuration: scanTotalDuration,
		profileDurations:  profileDurations,
		width:             width,
	}
}

func (m pickerModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter", " ":
			profile := m.profiles[m.cursor]
			return m, func() tea.Msg {
				return profileSelectedMsg{profile: profile, browse: false}
			}
		case "b", "B":
			profile := m.profiles[m.cursor]
			return m, func() tea.Msg {
				return profileSelectedMsg{profile: profile, browse: true}
			}
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init — profile picker"))
	b.WriteString("\n")
	if m.scanTotalDuration > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"scanned %d profiles in %s  (strict: %s)",
			len(m.profiles),
			durationLabel(m.scanTotalDuration),
			durationLabel(m.profileDurations["strict"]),
		)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	headers := []string{"Profile", "Findings", "Fixable", "Rules", "Top rules"}
	rows := make([][]string, 0, len(m.profiles))
	for _, p := range m.profiles {
		res := m.scans[p]
		if res == nil {
			rows = append(rows, []string{p, "?", "?", "?", ""})
			continue
		}
		top := res.TopRules(3)
		topStr := make([]string, 0, len(top))
		for _, t := range top {
			topStr = append(topStr, fmt.Sprintf("%s(%d)", t.Name, t.Count))
		}
		rows = append(rows, []string{
			p,
			fmt.Sprintf("%d", res.Total),
			fmt.Sprintf("%d", res.Fixable),
			fmt.Sprintf("%d", len(res.ByRule)),
			strings.Join(topStr, " "),
		})
	}
	b.WriteString(renderTable(headers, rows, m.cursor))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ move  enter select  q quit"))
	return b.String()
}
