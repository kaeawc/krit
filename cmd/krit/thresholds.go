package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kaeawc/krit/internal/onboarding"
)

// thresholdSpec describes one tunable numeric rule threshold.
type thresholdSpec struct {
	ruleset string
	rule    string
	field   string
	label   string
	min     int
	max     int
	step    int
}

var thresholdSpecs = []thresholdSpec{
	{"complexity", "LongMethod", "allowedLines", "LongMethod — max lines per function", 10, 500, 10},
	{"complexity", "CyclomaticComplexMethod", "allowedComplexity", "CyclomaticComplexMethod — max complexity", 2, 50, 1},
	{"complexity", "LongParameterList", "allowedFunctionParameters", "LongParameterList — max function params", 2, 20, 1},
	{"complexity", "LargeClass", "allowedLines", "LargeClass — max lines per class", 100, 3000, 50},
	{"complexity", "NestedBlockDepth", "allowedDepth", "NestedBlockDepth — max nesting", 1, 10, 1},
	{"complexity", "TooManyFunctions", "allowedFunctionsPerFile", "TooManyFunctions — max per file", 2, 80, 1},
	{"complexity", "ComplexCondition", "allowedConditions", "ComplexCondition — max boolean ops", 1, 10, 1},
	{"style", "ReturnCount", "max", "ReturnCount — max returns per function", 1, 10, 1},
	{"style", "ThrowsCount", "max", "ThrowsCount — max throws per function", 1, 10, 1},
	{"style", "MaxLineLength", "maxLineLength", "MaxLineLength — max columns per line", 60, 250, 10},
}

// thresholdsModel drives the threshold slider phase. It loads the
// current values from the selected profile YAML asynchronously (Phase 4)
// so the UI is never blocked by file I/O.
type thresholdsModel struct {
	selected  string
	repoRoot  string
	acceptAll bool

	values  []int
	cursor  int
	loaded  bool
}

// thresholdsLoadedMsg carries the values read from the profile YAML.
type thresholdsLoadedMsg struct {
	values []int
}

func newThresholdsModel(selected, repoRoot string, acceptAll bool) thresholdsModel {
	return thresholdsModel{
		selected:  selected,
		repoRoot:  repoRoot,
		acceptAll: acceptAll,
	}
}

// Init reads the profile YAML asynchronously.
func (m thresholdsModel) Init() tea.Cmd {
	selected := m.selected
	repoRoot := m.repoRoot
	return func() tea.Msg {
		values := make([]int, len(thresholdSpecs))
		profileYAML, err := os.ReadFile(onboarding.ProfilePath(repoRoot, selected))
		if err == nil {
			extracted := extractThresholdValues(profileYAML)
			for i, spec := range thresholdSpecs {
				key := spec.ruleset + "." + spec.rule + "." + spec.field
				if v, ok := extracted[key]; ok {
					values[i] = v
					continue
				}
				values[i] = (spec.min + spec.max) / 2
			}
		} else {
			for i, spec := range thresholdSpecs {
				values[i] = (spec.min + spec.max) / 2
			}
		}
		return thresholdsLoadedMsg{values: values}
	}
}

func (m thresholdsModel) Update(msg tea.Msg) (phaseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case thresholdsLoadedMsg:
		m.values = msg.values
		m.loaded = true
		if m.acceptAll {
			return m, m.commitCmd()
		}
		return m, nil

	case tea.KeyMsg:
		if !m.loaded {
			return m, nil
		}
		return m.updateKey(msg)
	}
	return m, nil
}

func (m thresholdsModel) updateKey(msg tea.KeyMsg) (phaseModel, tea.Cmd) {
	if len(thresholdSpecs) == 0 {
		return m, m.commitCmd()
	}
	spec := thresholdSpecs[m.cursor]
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(thresholdSpecs)-1 {
			m.cursor++
		}
	case "left", "h", "-":
		v := m.values[m.cursor] - spec.step
		if v < spec.min {
			v = spec.min
		}
		m.values[m.cursor] = v
	case "right", "l", "+", "=":
		v := m.values[m.cursor] + spec.step
		if v > spec.max {
			v = spec.max
		}
		m.values[m.cursor] = v
	case "enter", " ", "s":
		return m, m.commitCmd()
	}
	return m, nil
}

func (m thresholdsModel) commitCmd() tea.Cmd {
	overrides := make([]onboarding.ThresholdOverride, len(thresholdSpecs))
	for i, s := range thresholdSpecs {
		val := 0
		if i < len(m.values) {
			val = m.values[i]
		}
		overrides[i] = onboarding.ThresholdOverride{
			Ruleset: s.ruleset,
			Rule:    s.rule,
			Field:   s.field,
			Value:   val,
		}
	}
	return func() tea.Msg {
		return thresholdsDoneMsg{overrides: overrides}
	}
}

func (m thresholdsModel) View() string {
	if !m.loaded {
		return titleStyle.Render("krit init — thresholds") + "\n\n" +
			dimStyle.Render("loading profile values...")
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init — thresholds"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Adjust numeric thresholds for the main complexity/style rules. The generated krit.yml will include whatever values you set here."))
	b.WriteString("\n\n")
	for i, spec := range thresholdSpecs {
		val := m.values[i]
		bar := renderSliderBar(val, spec.min, spec.max, 30)
		line := fmt.Sprintf("%-45s %6d  %s", spec.label, val, bar)
		if i == m.cursor {
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

// extractThresholdValues walks a profile YAML and returns a flat map
// of ruleset.rule.field -> int for every threshold spec.
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

// yamlUnmarshal is a thin wrapper around yaml.v3 so this file does not
// directly import the YAML package.
func yamlUnmarshal(data []byte, out *map[string]interface{}) error {
	return onboarding.YAMLUnmarshalMap(data, out)
}
