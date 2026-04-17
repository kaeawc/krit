package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// startThresholds loads the current threshold values from the
// selected profile YAML and enters phaseThresholds. If --yes was
// passed, the phase is skipped and we transition straight to
// phaseWriting so CI runs don't stall.
func (m *initModel) startThresholds() {
	m.thresholdCursor = 0
	m.thresholdValues = make([]int, len(thresholdSpecs))

	profileYAML, err := os.ReadFile(onboarding.ProfilePath(m.opts.RepoRoot, m.selected))
	if err == nil {
		values := extractThresholdValues(profileYAML)
		for i, spec := range thresholdSpecs {
			key := spec.ruleset + "." + spec.rule + "." + spec.field
			if v, ok := values[key]; ok {
				m.thresholdValues[i] = v
				continue
			}
			// Fallback to the spec's lower-middle as a last resort;
			// this only kicks in if the profile file is missing or
			// malformed, which is an error elsewhere.
			m.thresholdValues[i] = (spec.min + spec.max) / 2
		}
	}

	if m.acceptAll {
		m.phase = phaseWriting
		return
	}
	m.phase = phaseThresholds
}

// extractThresholdValues walks a profile YAML and returns a
// flat map of ruleset.rule.field -> int for every threshold spec
// the TUI cares about. Missing values are simply absent from the
// returned map.
func extractThresholdValues(profileYAML []byte) map[string]int {
	out := make(map[string]int)
	var tree map[string]interface{}
	if err := yamlUnmarshal(profileYAML, &tree); err != nil {
		return out
	}
	for _, spec := range thresholdSpecs {
		rs, _ := tree[spec.ruleset].(map[string]interface{})
		if rs == nil {
			continue
		}
		rule, _ := rs[spec.rule].(map[string]interface{})
		if rule == nil {
			continue
		}
		if v, ok := toInt(rule[spec.field]); ok {
			out[spec.ruleset+"."+spec.rule+"."+spec.field] = v
		}
	}
	return out
}

func toInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	}
	return 0, false
}

// yamlUnmarshal is a small indirection so init.go does not directly
// import gopkg.in/yaml.v3 (the onboarding package already does it,
// keeping imports clean). This is a thin wrapper around yaml.v3.
func yamlUnmarshal(data []byte, out *map[string]interface{}) error {
	return onboarding.YAMLUnmarshalMap(data, out)
}

func (m initModel) updateThresholds(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(thresholdSpecs) == 0 {
		m.phase = phaseWriting
		return m, m.writeConfigCmd()
	}
	spec := thresholdSpecs[m.thresholdCursor]
	switch msg.String() {
	case "up", "k":
		if m.thresholdCursor > 0 {
			m.thresholdCursor--
		}
	case "down", "j":
		if m.thresholdCursor < len(thresholdSpecs)-1 {
			m.thresholdCursor++
		}
	case "left", "h", "-":
		v := m.thresholdValues[m.thresholdCursor] - spec.step
		if v < spec.min {
			v = spec.min
		}
		m.thresholdValues[m.thresholdCursor] = v
	case "right", "l", "+", "=":
		v := m.thresholdValues[m.thresholdCursor] + spec.step
		if v > spec.max {
			v = spec.max
		}
		m.thresholdValues[m.thresholdCursor] = v
	case "enter", " ", "s":
		// Write overrides and advance.
		m.thresholdOverrides = m.thresholdOverrides[:0]
		for i, s := range thresholdSpecs {
			m.thresholdOverrides = append(m.thresholdOverrides, onboarding.ThresholdOverride{
				Ruleset: s.ruleset,
				Rule:    s.rule,
				Field:   s.field,
				Value:   m.thresholdValues[i],
			})
		}
		m.phase = phaseWriting
		return m, m.writeConfigCmd()
	}
	return m, nil
}

func (m initModel) viewThresholds() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init — thresholds"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Adjust numeric thresholds for the main complexity/style rules. The generated krit.yml will include whatever values you set here."))
	b.WriteString("\n\n")

	for i, spec := range thresholdSpecs {
		val := m.thresholdValues[i]
		bar := renderSliderBar(val, spec.min, spec.max, 30)
		line := fmt.Sprintf("%-45s %6d  %s", spec.label, val, bar)
		if i == m.thresholdCursor {
			b.WriteString(selectedStyle.Render("▸ " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/↓ pick · ←/→ or +/- adjust · enter continue · q quit"))
	return b.String()
}

// renderSliderBar draws a simple text slider of the given width with
// the cursor at position val.
func renderSliderBar(val, min, max, width int) string {
	if max <= min {
		return strings.Repeat("─", width)
	}
	pos := (val - min) * (width - 1) / (max - min)
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}
	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i == pos:
			b.WriteString(selectedStyle.Render("●"))
		case i < pos:
			b.WriteString(dimStyle.Render("─"))
		default:
			b.WriteString(dimStyle.Render("·"))
		}
	}
	return b.String()
}
