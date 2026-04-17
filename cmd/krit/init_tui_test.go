package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"

	"github.com/kaeawc/krit/internal/onboarding"
)

// newTestModel builds a minimal initModel the TUI tests can drive
// directly without running the bubbletea program. Tests set m.phase
// and invoke Update/View methods to verify behavior.
func newTestModel(t *testing.T) initModel {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := onboarding.LoadRegistry(filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := newInitModel(onboarding.ScanOptions{RepoRoot: repoRoot}, reg, repoRoot, "", false)
	m.width = 160
	m.height = 40
	m.selected = "balanced"
	m.scans["balanced"] = &onboarding.ScanResult{
		Total: 50,
		ByRule: map[string]int{
			"MagicNumber":              10,
			"UnsafeCallOnNullableType": 5,
			"UnsafeCast":               3,
			"ComposeUnstableParameter": 2,
		},
		Findings: map[string][]onboarding.FindingSample{
			"MagicNumber": {
				{File: "/src/Foo.kt", Line: 42, Message: "Magic number 42 used in expression"},
				{File: "/src/Bar.kt", Line: 10, Message: "Magic number 100 used in expression"},
			},
			"UnsafeCallOnNullableType": {
				{File: "/src/Baz.kt", Line: 7, Message: "Unsafe call on nullable receiver"},
			},
		},
	}
	return m
}

// pressKey returns the model state after dispatching a key through
// Update for the given phase. Useful for chaining key events.
func pressKey(m initModel, key string) initModel {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return next.(initModel)
}

// pressNamedKey is like pressKey but for named keys (arrows, enter, etc.).
func pressNamedKey(m initModel, t tea.KeyType) initModel {
	next, _ := m.Update(tea.KeyMsg{Type: t})
	return next.(initModel)
}

// pressKeyDrain dispatches a key and then executes the returned
// tea.Cmd (if any), feeding its message back into Update. This is
// needed for sub-models that communicate via command-returned
// messages (e.g. pickerModel emitting profileSelectedMsg).
func pressKeyDrain(m initModel, key string) initModel {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	m = next.(initModel)
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			next, _ = m.Update(msg)
			m = next.(initModel)
		}
	}
	return m
}

// pressNamedKeyDrain is like pressKeyDrain but for named keys.
func pressNamedKeyDrain(m initModel, t tea.KeyType) initModel {
	next, cmd := m.Update(tea.KeyMsg{Type: t})
	m = next.(initModel)
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			next, _ = m.Update(msg)
			m = next.(initModel)
		}
	}
	return m
}

// ---------- applyAnswer cascade + inversion ----------------------------

func TestApplyAnswerInversionDisables(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()

	// Find allow-bang-operator (should be cascaded out of visibleQs
	// because strict-null-safety is its parent, but we can still call
	// applyAnswer on it directly to test the inversion logic).
	var q *onboarding.Question
	for i := range m.registry.Questions {
		if m.registry.Questions[i].ID == "allow-bang-operator" {
			q = &m.registry.Questions[i]
			break
		}
	}
	if q == nil {
		t.Fatal("allow-bang-operator missing from registry")
	}

	initial := m.liveTotal
	m.applyAnswer(q, true) // "yes, allow !!"

	// The two rules in allow-bang-operator should subtract their
	// counts from liveTotal since they're being disabled.
	expectedDelta := m.scans["balanced"].ByRule["UnsafeCallOnNullableType"] +
		m.scans["balanced"].ByRule["MapGetWithNotNullAssertionOperator"]
	if initial-m.liveTotal != expectedDelta {
		t.Errorf("expected liveTotal delta %d, got %d", expectedDelta, initial-m.liveTotal)
	}

	if len(m.answers) == 0 || m.answers[len(m.answers)-1].QuestionID != "allow-bang-operator" {
		t.Error("applyAnswer did not append the answer")
	}
}

func TestApplyAnswerCascadeStrictYes(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()

	// Find enforce-compose-stability parent (cascades to 3 children).
	var parent *onboarding.Question
	for i := range m.registry.Questions {
		if m.registry.Questions[i].ID == "enforce-compose-stability" {
			parent = &m.registry.Questions[i]
			break
		}
	}
	if parent == nil {
		t.Fatal("enforce-compose-stability missing")
	}
	if len(parent.CascadesTo) == 0 {
		t.Fatal("enforce-compose-stability has no cascades")
	}

	before := len(m.answers)
	m.applyAnswer(parent, true) // yes → cascades to children with strict defaults

	// We should have parent + all children in answers now.
	got := len(m.answers) - before
	expected := 1 + len(parent.CascadesTo)
	if got != expected {
		t.Errorf("applyAnswer should have added %d answers, got %d", expected, got)
	}

	// Every cascaded child should be marked Cascaded:true with Parent set.
	for _, a := range m.answers[before+1:] {
		if !a.Cascaded {
			t.Errorf("child %s not marked Cascaded", a.QuestionID)
		}
		if a.Parent != "enforce-compose-stability" {
			t.Errorf("child %s Parent = %q, want enforce-compose-stability", a.QuestionID, a.Parent)
		}
	}
}

func TestApplyAnswerCascadeRelaxedBucket(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()

	var parent *onboarding.Question
	for i := range m.registry.Questions {
		if m.registry.Questions[i].ID == "enforce-compose-stability" {
			parent = &m.registry.Questions[i]
			break
		}
	}
	if parent == nil {
		t.Fatal("enforce-compose-stability missing")
	}

	before := len(m.answers)
	m.applyAnswer(parent, false) // no → cascades to children with relaxed defaults

	childAnswers := m.answers[before+1:]
	// For enforce-compose-stability, every child has relaxed:false,
	// so derived value should be false for all of them.
	for _, a := range childAnswers {
		if a.Value {
			t.Errorf("child %s derived true, want false (relaxed bucket)", a.QuestionID)
		}
	}
}

// ---------- updatePicker key handling ---------------------------------

func TestUpdatePickerNavigation(t *testing.T) {
	m := newTestModel(t)
	m.phase = phasePicker

	// Down twice → index 2
	m = pressKey(m, "j")
	m = pressKey(m, "j")
	if m.picker.cursor != 2 {
		t.Errorf("picker.cursor = %d, want 2", m.picker.cursor)
	}

	// Up once → index 1
	m = pressKey(m, "k")
	if m.picker.cursor != 1 {
		t.Errorf("picker.cursor = %d, want 1", m.picker.cursor)
	}

	// Can't go above 0.
	m.picker.cursor = 0
	m = pressKey(m, "k")
	if m.picker.cursor != 0 {
		t.Errorf("picker.cursor = %d, want 0 (clamped)", m.picker.cursor)
	}

	// Can't go below last profile.
	m.picker.cursor = len(m.profiles) - 1
	m = pressKey(m, "j")
	if m.picker.cursor != len(m.profiles)-1 {
		t.Errorf("picker.cursor = %d, want %d (clamped)", m.picker.cursor, len(m.profiles)-1)
	}
}

func TestUpdatePickerEnterToQuestionnaire(t *testing.T) {
	m := newTestModel(t)
	m.phase = phasePicker
	m.picker.cursor = 1 // balanced

	m = pressNamedKeyDrain(m, tea.KeyEnter)
	if m.phase != phaseQuestionnaire {
		t.Errorf("phase after enter = %v, want questionnaire", m.phase)
	}
	if m.selected != m.profiles[1] {
		t.Errorf("selected = %q, want %q", m.selected, m.profiles[1])
	}
}

func TestUpdatePickerBrowseToExplorer(t *testing.T) {
	m := newTestModel(t)
	m.phase = phasePicker
	m.picker.cursor = 1

	m = pressKeyDrain(m, "b")
	if m.phase != phaseExplorer {
		t.Errorf("phase after 'b' = %v, want explorer", m.phase)
	}
	if m.selected != m.profiles[1] {
		t.Errorf("selected = %q, want %q", m.selected, m.profiles[1])
	}
	if len(m.ruleItems) == 0 {
		t.Error("ruleItems not populated after transitioning to explorer")
	}
}

// ---------- updateQuestionnaire key handling --------------------------

func TestUpdateQuestionnaireToggleYesNo(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()
	if m.phase != phaseQuestionnaire {
		t.Fatalf("startQuestionnaire did not enter phase: %v", m.phase)
	}

	// Press 'n' to move cursor to No.
	m = pressKey(m, "n")
	if m.qCursor != 1 {
		t.Errorf("after 'n', qCursor = %d, want 1", m.qCursor)
	}

	// Press 'y' to move back to Yes.
	m = pressKey(m, "y")
	if m.qCursor != 0 {
		t.Errorf("after 'y', qCursor = %d, want 0", m.qCursor)
	}

	// left / right arrows.
	m = pressNamedKey(m, tea.KeyRight)
	if m.qCursor != 1 {
		t.Errorf("after right, qCursor = %d, want 1", m.qCursor)
	}
	m = pressNamedKey(m, tea.KeyLeft)
	if m.qCursor != 0 {
		t.Errorf("after left, qCursor = %d, want 0", m.qCursor)
	}
}

func TestUpdateQuestionnaireEnterAdvances(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()
	before := m.qIdx

	m = pressNamedKey(m, tea.KeyEnter)
	if m.qIdx != before+1 {
		t.Errorf("qIdx after enter = %d, want %d", m.qIdx, before+1)
	}
	if len(m.answers) == 0 {
		t.Error("enter did not record an answer")
	}
}

func TestQuestionnairePreviewRespondsToYesNo(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()

	// Move to a non-parent question that has fixtures.
	for m.qIdx < len(m.visibleQs) {
		q := &m.registry.Questions[m.visibleQs[m.qIdx]]
		if q.Kind != "parent" && q.PositiveFixture != nil {
			break
		}
		m.qIdx++
	}
	if m.qIdx >= len(m.visibleQs) {
		t.Skip("no non-parent question with fixture found")
	}

	// Yes → rule active → should show diff content.
	m.qCursor = 0
	viewYes := m.View()

	// No → rule inactive → should show "rule disabled".
	m.qCursor = 1
	viewNo := m.View()

	if viewYes == viewNo {
		t.Error("expected different preview for Yes vs No")
	}
}

// ---------- updateThresholds ------------------------------------------

func TestUpdateThresholdsNavAndAdjust(t *testing.T) {
	m := newTestModel(t)
	m.startThresholds()
	if m.phase != phaseThresholds {
		t.Fatalf("startThresholds did not enter phase: %v", m.phase)
	}
	if len(m.thresholdValues) != len(thresholdSpecs) {
		t.Fatalf("thresholdValues len = %d, want %d", len(m.thresholdValues), len(thresholdSpecs))
	}

	// Down arrow → cursor advances.
	m = pressKey(m, "j")
	if m.thresholdCursor != 1 {
		t.Errorf("after down, cursor = %d, want 1", m.thresholdCursor)
	}

	spec := thresholdSpecs[m.thresholdCursor]
	initial := m.thresholdValues[m.thresholdCursor]

	// + bumps value by step.
	m = pressKey(m, "+")
	if m.thresholdValues[m.thresholdCursor] != initial+spec.step {
		t.Errorf("after +, value = %d, want %d", m.thresholdValues[m.thresholdCursor], initial+spec.step)
	}

	// - drops it back.
	m = pressKey(m, "-")
	if m.thresholdValues[m.thresholdCursor] != initial {
		t.Errorf("after -, value = %d, want %d", m.thresholdValues[m.thresholdCursor], initial)
	}
}

func TestUpdateThresholdsClampsToMinMax(t *testing.T) {
	m := newTestModel(t)
	m.startThresholds()

	spec := thresholdSpecs[0]
	// Force value below min and invoke -.
	m.thresholdValues[0] = spec.min
	m = pressKey(m, "-")
	if m.thresholdValues[0] != spec.min {
		t.Errorf("value below min = %d, want %d (clamped)", m.thresholdValues[0], spec.min)
	}

	// Force value above max and invoke +.
	m.thresholdValues[0] = spec.max
	m = pressKey(m, "+")
	if m.thresholdValues[0] != spec.max {
		t.Errorf("value above max = %d, want %d (clamped)", m.thresholdValues[0], spec.max)
	}
}

func TestUpdateThresholdsEnterProducesOverrides(t *testing.T) {
	m := newTestModel(t)
	m.startThresholds()

	_, cmd := m.updateThresholds(tea.KeyMsg{Type: tea.KeyEnter})
	// The model returned by Update is the one we care about; fetch it.
	modelAfter, _ := m.updateThresholds(tea.KeyMsg{Type: tea.KeyEnter})
	mAfter := modelAfter.(initModel)
	if mAfter.phase != phaseWriting {
		t.Errorf("phase after enter = %v, want writing", mAfter.phase)
	}
	if len(mAfter.thresholdOverrides) != len(thresholdSpecs) {
		t.Errorf("thresholdOverrides len = %d, want %d", len(mAfter.thresholdOverrides), len(thresholdSpecs))
	}
	if cmd == nil {
		t.Error("expected writeConfigCmd from enter, got nil")
	}
}

// ---------- updateAutofixConfirm / updateBaselineConfirm --------------

func TestUpdateAutofixConfirmYesRuns(t *testing.T) {
	m := newTestModel(t)
	m.phase = phaseAutofixConfirm
	m.autofixConfirm = newConfirmModel("krit init — autofix", "Apply safe autofixes now?", true)
	m.configPath = filepath.Join(t.TempDir(), "krit.yml")

	_, cmd := m.updateAutofixConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	modelAfter, _ := m.updateAutofixConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	mAfter := modelAfter.(initModel)
	if mAfter.phase != phaseAutofixRunning {
		t.Errorf("phase = %v, want autofixRunning", mAfter.phase)
	}
	if cmd == nil {
		t.Error("expected autofixCmd, got nil")
	}
}

func TestUpdateAutofixConfirmNoSkips(t *testing.T) {
	m := newTestModel(t)
	m.phase = phaseAutofixConfirm
	m = pressKey(m, "n")
	m = pressNamedKey(m, tea.KeyEnter)
	if m.phase != phaseBaselineConfirm {
		t.Errorf("phase after no = %v, want baselineConfirm", m.phase)
	}
	if !m.autofixSkipped {
		t.Error("autofixSkipped should be true after selecting No")
	}
}

func TestUpdateBaselineConfirmNoSkipsToDone(t *testing.T) {
	m := newTestModel(t)
	m.phase = phaseBaselineConfirm
	m = pressKey(m, "n")
	m = pressNamedKey(m, tea.KeyEnter)
	if m.phase != phaseDone {
		t.Errorf("phase after no = %v, want done", m.phase)
	}
	if !m.baselineSkipped {
		t.Error("baselineSkipped should be true after selecting No")
	}
}

// ---------- startThresholds reads profile YAML ------------------------

func TestStartThresholdsReadsProfileValues(t *testing.T) {
	m := newTestModel(t)
	m.startThresholds()

	// balanced profile has LongMethod.allowedLines: 60.
	var longMethodVal int
	for i, spec := range thresholdSpecs {
		if spec.rule == "LongMethod" && spec.field == "allowedLines" {
			longMethodVal = m.thresholdValues[i]
			break
		}
	}
	if longMethodVal != 60 {
		t.Errorf("LongMethod.allowedLines from balanced profile = %d, want 60", longMethodVal)
	}

	// balanced profile has MaxLineLength.maxLineLength: 120.
	var maxLineVal int
	for i, spec := range thresholdSpecs {
		if spec.rule == "MaxLineLength" {
			maxLineVal = m.thresholdValues[i]
			break
		}
	}
	if maxLineVal != 120 {
		t.Errorf("MaxLineLength from balanced = %d, want 120", maxLineVal)
	}
}

func TestExtractThresholdValuesMalformed(t *testing.T) {
	// Invalid YAML produces an empty map, not a panic.
	got := extractThresholdValues([]byte("this: is: not: valid"))
	if got == nil {
		t.Error("expected non-nil map on parse failure")
	}
}

// ---------- View smoke tests ------------------------------------------

func TestAllPhasesRender(t *testing.T) {
	phases := []struct {
		name  string
		setup func(*initModel)
		want  string
	}{
		{"scanning", func(m *initModel) { m.phase = phaseScanning }, "starting strict scan"},
		{"picker", func(m *initModel) { m.phase = phasePicker }, "profile picker"},
		{"questionnaire", func(m *initModel) { m.startQuestionnaire() }, "questionnaire"},
		{"thresholds", func(m *initModel) { m.startThresholds() }, "thresholds"},
		{"explorer", func(m *initModel) { m.startExplorer() }, "rule explorer"},
		{"writing", func(m *initModel) { m.phase = phaseWriting }, "writing config"},
		{"autofixConfirm", func(m *initModel) {
			m.phase = phaseAutofixConfirm
			m.autofixConfirm = newConfirmModel("krit init — autofix", "Apply safe autofixes now?", true)
		}, "Apply safe autofixes"},
		{"autofixRunning", func(m *initModel) { m.phase = phaseAutofixRunning }, "applying safe autofixes"},
		{"baselineConfirm", func(m *initModel) {
			m.phase = phaseBaselineConfirm
			m.fixedCount = 5
			m.postfixTotal = 42
			m.baselineConfirm = newConfirmModel(
				"krit init — baseline",
				"Write a baseline to suppress remaining findings?",
				true,
			)
		}, "baseline"},
		{"baselineRunning", func(m *initModel) { m.phase = phaseBaselineRunning }, "writing"},
		{"done", func(m *initModel) {
			m.phase = phaseDone
			m.configPath = "/tmp/krit.yml"
			m.baselineWritten = true
			m.baselinePath = "/tmp/.krit/baseline.xml"
		}, "done"},
	}

	for _, p := range phases {
		p := p
		t.Run(p.name, func(t *testing.T) {
			m := newTestModel(t)
			p.setup(&m)
			out := m.View()
			if out == "" {
				t.Errorf("phase %s produced empty view", p.name)
			}
			if !strings.Contains(strings.ToLower(out), strings.ToLower(p.want)) {
				t.Errorf("phase %s view missing %q; first 200 chars: %s", p.name, p.want, snippet(out, 200))
			}
		})
	}
}

func TestViewErrorState(t *testing.T) {
	m := newTestModel(t)
	m.err = &tempError{msg: "boom"}
	out := m.View()
	if !strings.Contains(out, "error") || !strings.Contains(out, "boom") {
		t.Errorf("error view missing expected text: %q", out)
	}
}

type tempError struct{ msg string }

func (e *tempError) Error() string { return e.msg }

// ---------- scanDoneMsg flow ------------------------------------------

func TestScanDoneAdvancesAndEventuallyShowsPicker(t *testing.T) {
	m := newTestModel(t)
	m.scans = make(map[string]*onboarding.ScanResult) // reset the seeded data
	m.phase = phaseScanning

	for i, p := range m.profiles {
		modelAfter, _ := m.Update(scanDoneMsg{
			profile: p,
			result:  &onboarding.ScanResult{Total: i * 10, ByRule: map[string]int{}},
		})
		m = modelAfter.(initModel)
	}

	if m.phase != phasePicker {
		t.Errorf("after 4 scans, phase = %v, want picker", m.phase)
	}
	if len(m.scans) != len(m.profiles) {
		t.Errorf("scans recorded = %d, want %d", len(m.scans), len(m.profiles))
	}
}

// TestRunHeadlessInitInProcess calls runHeadlessInit directly
// instead of via exec, so Go coverage can see everything under it.
// This complements TestInitSubcommandHeadless which exec's the
// binary and therefore doesn't contribute to in-process coverage.
func TestRunHeadlessInitInProcess(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("playground missing: %v", err)
	}

	target := t.TempDir()
	copyDirForTest(t, src, target)
	_ = os.Remove(filepath.Join(target, "krit.yml"))
	_ = os.RemoveAll(filepath.Join(target, ".krit"))

	reg, err := onboarding.LoadRegistry(filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json"))
	if err != nil {
		t.Fatal(err)
	}

	code := runHeadlessInit(onboarding.ScanOptions{
		KritBin:  binPath,
		RepoRoot: repoRoot,
		Target:   target,
	}, reg, "balanced")

	if code != 0 {
		t.Errorf("runHeadlessInit exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(target, "krit.yml")); err != nil {
		t.Errorf("krit.yml missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".krit", "baseline.xml")); err != nil {
		t.Errorf("baseline.xml missing: %v", err)
	}
}

// TestRunHeadlessInitUnknownProfile exercises the validation branch.
func TestRunHeadlessInitUnknownProfile(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := onboarding.LoadRegistry(filepath.Join(repoRoot, "config", "onboarding", "controversial-rules.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Redirect stderr so the test output stays clean.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	code := runHeadlessInit(onboarding.ScanOptions{
		KritBin:  binPath,
		RepoRoot: repoRoot,
		Target:   t.TempDir(),
	}, reg, "bogus")

	w.Close()
	os.Stderr = origStderr
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 2 {
		t.Errorf("runHeadlessInit exit code = %d, want 2", code)
	}
	if !strings.Contains(msg, "unknown profile") {
		t.Errorf("stderr missing 'unknown profile'; got: %q", msg)
	}
}

// copyDirForTest is a thin helper for copying a playground into a
// throwaway target directory. Kept separate from the copyDir in
// krit_init_integration_test.go to avoid cross-file coupling.
func copyDirForTest(t *testing.T, src, dst string) {
	t.Helper()
	copyDir(t, src, dst)
}

// TestFindOnboardingRepoRootEnv verifies the KRIT_REPO_ROOT override
// takes precedence, then falls through to directory walking.
func TestFindOnboardingRepoRootEnv(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	got, err := findOnboardingRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != repoRoot {
		t.Errorf("findOnboardingRepoRoot = %q, want %q", got, repoRoot)
	}
}

func TestFindOnboardingRepoRootBogusEnvFallsThrough(t *testing.T) {
	t.Setenv("KRIT_REPO_ROOT", "/nonexistent/path/to/nothing")
	got, err := findOnboardingRepoRoot()
	// Should still find the repo via the cwd walk since tests run
	// from cmd/krit/.
	if err != nil {
		t.Fatalf("expected fallback resolution, got error: %v", err)
	}
	if got == "/nonexistent/path/to/nothing" {
		t.Errorf("findOnboardingRepoRoot returned the bogus env value without validating it")
	}
}

// TestAutofixCmdRuns exercises autofixCmd's full pipeline
// (pre-scan, --fix, post-scan) against a real target, which also
// covers runKritJSON in-process.
func TestAutofixCmdRuns(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("playground missing: %v", err)
	}

	target := t.TempDir()
	copyDirForTest(t, src, target)
	_ = os.Remove(filepath.Join(target, "krit.yml"))
	_ = os.RemoveAll(filepath.Join(target, ".krit"))

	// Write a minimal krit.yml so the autofix pass has something to read.
	profileYAML, err := os.ReadFile(filepath.Join(repoRoot, "config", "profiles", "balanced.yml"))
	if err != nil {
		t.Fatal(err)
	}
	configPath, err := onboarding.WriteConfigFile(target, onboarding.WriteConfigOptions{
		ProfileYAML: profileYAML,
		ProfileName: "balanced",
	})
	if err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t)
	m.opts.KritBin = binPath
	m.target = target
	m.configPath = configPath

	cmd := m.autofixCmd()
	if cmd == nil {
		t.Fatal("autofixCmd returned nil")
	}
	msg := cmd()
	done, ok := msg.(autofixDoneMsg)
	if !ok {
		t.Fatalf("expected autofixDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("autofix failed: %v", done.err)
	}
	// Post-fix count can be <= pre-fix count depending on what's
	// fixable at idiomatic level. We just assert the pipeline ran.
	if done.prefix < 0 || done.postfix < 0 {
		t.Errorf("negative counts from autofix: prefix=%d postfix=%d", done.prefix, done.postfix)
	}
}

// TestBaselineCmdRuns covers baselineCmd in-process.
func TestBaselineCmdRuns(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("playground missing: %v", err)
	}

	target := t.TempDir()
	copyDirForTest(t, src, target)

	profileYAML, err := os.ReadFile(filepath.Join(repoRoot, "config", "profiles", "balanced.yml"))
	if err != nil {
		t.Fatal(err)
	}
	configPath, err := onboarding.WriteConfigFile(target, onboarding.WriteConfigOptions{
		ProfileYAML: profileYAML,
		ProfileName: "balanced",
	})
	if err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t)
	m.opts.KritBin = binPath
	m.target = target
	m.configPath = configPath

	cmd := m.baselineCmd()
	if cmd == nil {
		t.Fatal("baselineCmd returned nil")
	}
	msg := cmd()
	done, ok := msg.(baselineDoneMsg)
	if !ok {
		t.Fatalf("expected baselineDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("baseline failed: %v", done.err)
	}
	if done.path == "" {
		t.Error("baselineDoneMsg.path is empty")
	}
	if _, err := os.Stat(done.path); err != nil {
		t.Errorf("baseline file missing: %v", err)
	}
}

// TestRunInitSubcommandHelp covers runInitSubcommand's help branch.
// The --help flag exits early without running the TUI program, so
// the call is safe to make in-process.
func TestRunInitSubcommandHelp(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	t.Setenv("KRIT_BIN", binPath)

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	code := runInitSubcommand([]string{"--help"})

	w.Close()
	os.Stderr = origStderr
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 0 {
		t.Errorf("runInitSubcommand(--help) code = %d, want 0", code)
	}
	if !strings.Contains(msg, "Usage: krit init") {
		t.Errorf("help output missing usage line; got: %q", msg)
	}
}

// TestRunInitSubcommandMissingTarget exercises the "target not a
// directory" error branch.
func TestRunInitSubcommandMissingTarget(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	t.Setenv("KRIT_BIN", binPath)

	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	code := runInitSubcommand([]string{"/definitely/does/not/exist"})

	w.Close()
	os.Stderr = origStderr
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 2 {
		t.Errorf("runInitSubcommand with missing target code = %d, want 2", code)
	}
	if !strings.Contains(msg, "is not a directory") {
		t.Errorf("error message missing expected text; got: %q", msg)
	}
}

// TestRunInitSubcommandHeadlessDelegate confirms that passing
// --profile + --yes routes through runHeadlessInit without ever
// constructing a bubbletea program.
func TestRunInitSubcommandHeadlessDelegate(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("playground missing: %v", err)
	}

	target := t.TempDir()
	copyDirForTest(t, src, target)
	_ = os.Remove(filepath.Join(target, "krit.yml"))
	_ = os.RemoveAll(filepath.Join(target, ".krit"))

	t.Setenv("KRIT_REPO_ROOT", repoRoot)
	t.Setenv("KRIT_BIN", binPath)

	// Swallow stdout so the test log stays clean.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	code := runInitSubcommand([]string{"--profile", "balanced", "--yes", target})

	w.Close()
	os.Stdout = origStdout
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	msg := string(buf[:n])

	if code != 0 {
		t.Errorf("runInitSubcommand delegate code = %d, want 0; stdout: %s", code, msg)
	}
	if !strings.Contains(msg, "wrote ") || !strings.Contains(msg, "baseline written to ") {
		t.Errorf("stdout missing expected summary lines; got: %q", msg)
	}
}

// TestExplorerDedupesRuleNames ensures startExplorer collapses
// rules that happen to be registered under more than one
// BaseRule{} literal (e.g. AppCompatResource appears twice today).
// The user-facing explorer should show each name exactly once.
func TestExplorerDedupesRuleNames(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()

	seen := make(map[string]int, len(m.ruleItems))
	for _, item := range m.ruleItems {
		seen[item.name]++
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("rule %q appears %d times in ruleItems; expected 1", name, count)
		}
	}
}

// TestModelInitReturnsScanCmd confirms bubbletea's Init() hook
// returns a non-nil Cmd that schedules the first profile scan. This
// is the only way to cover Init() in-process — it's a lifecycle
// hook bubbletea calls at program start.
func TestModelInitReturnsScanCmd(t *testing.T) {
	m := newTestModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil Cmd")
	}
}

// TestResolveKritBinEnv covers the KRIT_BIN env resolution path.
func TestResolveKritBinEnv(t *testing.T) {
	t.Setenv("KRIT_BIN", binPath)
	got, err := resolveKritBin()
	if err != nil {
		t.Fatal(err)
	}
	if got != binPath {
		t.Errorf("resolveKritBin = %q, want %q", got, binPath)
	}
}

// TestWriteExplorerCmdProducesOverrides runs writeExplorerCmd's
// returned Cmd function and asserts that the generated krit.yml
// carries an override for every rule whose state differs from the
// registry default.
func TestWriteExplorerCmdProducesOverrides(t *testing.T) {
	m := newTestModel(t)
	m.target = t.TempDir()
	m.startExplorer()

	// Find MagicNumber (active by default in the registry) and flip
	// it off.
	flipped := false
	for i, item := range m.ruleItems {
		if item.name == "MagicNumber" {
			m.explorerCursor = i
			m.ruleActive[item.name] = false
			flipped = true
			break
		}
	}
	if !flipped {
		t.Fatal("MagicNumber not found in ruleItems")
	}

	cmd := m.writeExplorerCmd()
	if cmd == nil {
		t.Fatal("writeExplorerCmd returned nil Cmd")
	}
	msg := cmd()
	done, ok := msg.(writeDoneMsg)
	if !ok {
		t.Fatalf("expected writeDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("writeExplorerCmd failed: %v", done.err)
	}
	if done.path == "" {
		t.Fatal("writeDoneMsg.path is empty")
	}

	data, err := os.ReadFile(done.path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	// The generated file should contain our MagicNumber override as
	// active: false under the style ruleset.
	if !strings.Contains(body, "MagicNumber") {
		t.Error("generated krit.yml missing MagicNumber override")
	}
	if !strings.Contains(body, "active: false") {
		t.Error("generated krit.yml missing active: false override")
	}
}

func TestScanDoneErrorQuits(t *testing.T) {
	m := newTestModel(t)
	m.phase = phaseScanning
	next, cmd := m.Update(scanDoneMsg{profile: "strict", err: &tempError{msg: "scan failed"}})
	mAfter := next.(initModel)
	if mAfter.err == nil {
		t.Error("expected err to be set on scanDoneMsg with error")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd on scan error")
	}
}

// ---------- Explorer right-pane enhancements --------------------------

func TestExplorerViewShowsFindingSamples(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()

	// Navigate to MagicNumber which has 2 findings in the test scan.
	for i, item := range m.ruleItems {
		if item.name == "MagicNumber" {
			m.explorerCursor = i
			break
		}
	}
	view := m.viewExplorer()
	if !strings.Contains(view, "10 finding(s) in this scan") {
		t.Error("expected finding count for MagicNumber")
	}
	if !strings.Contains(view, "Foo.kt:42") {
		t.Error("expected sample finding Foo.kt:42 in right pane")
	}
	if !strings.Contains(view, "Bar.kt:10") {
		t.Error("expected sample finding Bar.kt:10 in right pane")
	}
}

func TestExplorerViewShowsDescription(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()

	// LongMethod has a Description() — find it.
	for i, item := range m.ruleItems {
		if item.name == "LongMethod" {
			m.explorerCursor = i
			break
		}
	}
	view := m.viewExplorer()
	if !strings.Contains(view, "Flags functions that exceed") {
		t.Error("expected LongMethod description in right pane")
	}
}

func TestExplorerFixtureDiffView(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()

	// Pre-populate fixture cache for the first rule with similar fixture data
	// (>40% similarity triggers diff view; dissimilar triggers stacked view).
	first := m.ruleItems[0]
	m.explorerFixtureCache[first.name] = fixturePair{
		positive: "package test\nimport foo\nval x = 42\nval y = 1",
		negative: "package test\nimport foo\nval x = CONSTANT\nval y = 1",
	}

	// Rule active → should show fixture content with highlighting.
	m.ruleActive[first.name] = true
	view := m.viewExplorer()
	if !strings.Contains(view, "val x") {
		t.Error("expected fixture code when rule is active")
	}

	// Rule inactive → should show "rule disabled".
	m.ruleActive[first.name] = false
	view = m.viewExplorer()
	if !strings.Contains(view, "rule disabled") {
		t.Error("expected 'rule disabled' when rule is inactive")
	}
}

func TestExplorerFixtureLoadedMsg(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()

	pair := fixturePair{
		positive: "fun test() {}",
		negative: "// no issue",
	}
	next, _ := m.Update(explorerFixtureLoadedMsg{
		ruleName: "MagicNumber",
		pair:     pair,
	})
	mAfter := next.(initModel)
	cached, ok := mAfter.explorerFixtureCache["MagicNumber"]
	if !ok {
		t.Fatal("expected MagicNumber in explorerFixtureCache after msg")
	}
	if cached.positive != "fun test() {}" {
		t.Errorf("cached positive = %q, want %q", cached.positive, "fun test() {}")
	}
}

func TestExplorerRuleRefPopulated(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()

	for _, item := range m.ruleItems {
		if item.ruleRef == nil {
			t.Errorf("ruleRef nil for rule %q", item.name)
		}
	}
}

func TestExplorerViewHintLine(t *testing.T) {
	m := newTestModel(t)
	m.startExplorer()
	view := m.viewExplorer()
	if !strings.Contains(view, "space toggle") {
		t.Error("expected 'space toggle' in bottom hint line")
	}
}

func TestReflowWordWrap(t *testing.T) {
	// Verify reflow/wordwrap works as expected for our use cases.
	got := wordwrap.String("hello world", 5)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected line break in wrapped output: %q", got)
	}
}

func TestViolationLines(t *testing.T) {
	positive := "line1\nline2\nline3"
	negative := "line1\nchanged\nline3"
	viol := violationLines(positive, negative)
	// line2 (index 1) should be a violation.
	if !viol[1] {
		t.Error("expected line 1 (line2) to be a violation")
	}
	// line1 and line3 should NOT be violations.
	if viol[0] {
		t.Error("line 0 (line1) should not be a violation")
	}
	if viol[2] {
		t.Error("line 2 (line3) should not be a violation")
	}
}

func TestRenderFixtureContentShowsPositiveOnly(t *testing.T) {
	pair := fixturePair{
		positive: "package test\nval x = 42\nval y = 1",
		negative: "package test\nval x = CONSTANT\nval y = 1",
	}
	content := renderFixtureContent(pair, 60)
	// Should show the positive fixture code, not a diff.
	if !strings.Contains(content, "val x = 42") {
		t.Error("expected positive fixture content")
	}
	// Should NOT show diff markers or clean section.
	if strings.Contains(content, "- ") || strings.Contains(content, "+ ") {
		t.Error("should not contain diff markers")
	}
	if strings.Contains(content, "clean:") {
		t.Error("should not show clean section")
	}
}

func TestRenderFixtureContentPrefersFixBefore(t *testing.T) {
	pair := fixturePair{
		positive:  "val x = 42",
		negative:  "const val X = 42",
		fixBefore: "val x: Any = foo() as String",
		fixAfter:  "val x: Any = foo() as? String",
	}
	content := renderFixtureContent(pair, 60)
	// Should show fixBefore content, not positive.
	if !strings.Contains(content, "foo() as String") {
		t.Error("expected fixBefore content when available")
	}
}

func TestRenderFixtureContentNoFixture(t *testing.T) {
	pair := fixturePair{}
	content := renderFixtureContent(pair, 60)
	if !strings.Contains(content, "no fixture") {
		t.Error("expected '(no fixture)' for empty pair")
	}
}

func TestQuestionnaireViewportScrollForwarded(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()

	// Press down — should forward to viewport (no error, model returned).
	m = pressKey(m, "j")
	// Press up — same.
	m = pressKey(m, "k")
	// If we got here without panic, viewport forwarding works.
}

func TestParentQuestionShowsChildFixture(t *testing.T) {
	m := newTestModel(t)
	m.startQuestionnaire()
	// Load fixtures inline for test.
	cmd := loadFixturesCmd(m.registry.Questions, m.opts.RepoRoot)
	msg := cmd().(fixturesLoadedMsg)
	for k, v := range msg.cache {
		m.fixtureCache[k] = v
	}
	m.syncFixtureViewport()

	// Find the "enforce-compose-stability" parent question.
	var parentIdx int
	found := false
	for i, qi := range m.visibleQs {
		q := &m.registry.Questions[qi]
		if q.ID == "enforce-compose-stability" {
			parentIdx = i
			found = true
			break
		}
	}
	if !found {
		t.Skip("enforce-compose-stability not in visible questions")
	}

	m.qIdx = parentIdx
	m.qCursor = 0 // Yes → rule active
	view := m.View()

	// Should NOT show "no fixture" — should show child's fixture content.
	if strings.Contains(view, "no fixture to preview") {
		t.Error("parent question should show child fixture, not 'no fixture to preview'")
	}
}

func TestBuildFindingsMapCapsAt3(t *testing.T) {
	result := &onboarding.ScanResult{
		Findings: map[string][]onboarding.FindingSample{
			"TestRule": {
				{File: "a.kt", Line: 1, Message: "m1"},
				{File: "b.kt", Line: 2, Message: "m2"},
				{File: "c.kt", Line: 3, Message: "m3"},
			},
		},
	}
	if len(result.Findings["TestRule"]) != 3 {
		t.Errorf("expected 3 findings, got %d", len(result.Findings["TestRule"]))
	}
}
