package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// pickerModel encapsulates the profile-picker phase of krit init.
// It owns the cursor state and delegates rendering via View, which
// receives scan data from the parent model through pickerViewData.
type pickerModel struct {
	profiles []string
	cursor   int
}

// profileSelectedMsg is emitted when the user commits a profile
// selection. browse=true indicates the user pressed 'b' to enter
// the rule explorer instead of the guided questionnaire.
type profileSelectedMsg struct {
	profile string
	browse  bool
}

// pickerViewData carries read-only data from the parent model that
// the picker needs to render the comparison table.
type pickerViewData struct {
	scans             map[string]*onboarding.ScanResult
	scanTotalDuration time.Duration
	profileDurations  map[string]time.Duration
}

func newPickerModel(profiles []string) pickerModel {
	return pickerModel{
		profiles: profiles,
		cursor:   0,
	}
}

// Update handles key messages for the picker phase. When the user
// selects a profile (enter or 'b'), it returns a profileSelectedMsg
// as a tea.Cmd.
func (m pickerModel) Update(msg tea.KeyMsg) (pickerModel, tea.Cmd) {
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
	return m, nil
}

// View renders the profile picker screen. The caller passes scan
// data and terminal width so the sub-model stays decoupled from
// the parent's full state.
func (m pickerModel) View(width int, data pickerViewData) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init — profile picker"))
	b.WriteString("\n")
	if data.scanTotalDuration > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"scanned %d profiles in %s  (strict: %s)",
			len(m.profiles),
			durationLabel(data.scanTotalDuration),
			durationLabel(data.profileDurations["strict"]),
		)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Build comparison table rows.
	headers := []string{"Profile", "Findings", "Fixable", "Rules", "Top rules"}
	rows := make([][]string, 0, len(m.profiles))
	for _, p := range m.profiles {
		res := data.scans[p]
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
