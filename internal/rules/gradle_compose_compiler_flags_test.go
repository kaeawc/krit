package rules_test

import (
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

const composeFlagsScript = `plugins {
    id("org.jetbrains.kotlin.plugin.compose")
}

composeCompiler {
    enableStrongSkippingMode = true
    enableIntrinsicRemember.set(true)
    enableNonSkippingGroupOptimization = true
    stabilityConfigurationFile = rootProject.layout.projectDirectory.file("stability.conf")
    generateFunctionKeyMetaClasses = false
    stabilityConfigurationFiles.add(rootProject.layout.projectDirectory.file("stability.conf"))
    featureFlags = setOf(
        ComposeFeatureFlag.StrongSkipping.disabled(),
        ComposeFeatureFlag.IntrinsicRemember,
        ComposeFeatureFlag.OptimizeNonSkippingGroups,
        ComposeFeatureFlag.PausableComposition,
    )
}
// composeCompiler { enableStrongSkippingMode = true }
`

func TestComposeCompilerRemovedFlagsByVersion(t *testing.T) {
	cases := []struct {
		kotlin   string
		lines    []int
		severity string
		conf     float64
	}{
		{"2.4.0", []int{6, 7, 8, 9, 10, 13, 14, 15, 16}, "error", api.ConfidenceHigh},
		{"2.3.0", []int{6, 7, 8, 9, 13, 14}, "warning", api.ConfidenceHigh},
		{"2.1.0", []int{6, 7, 8, 9}, "warning", api.ConfidenceHigh},
		{"2.0.20", []int{6, 7, 8}, "warning", api.ConfidenceHigh},
		{"2.0.10", nil, "", 0},
	}
	for _, c := range cases {
		fs := kotlinGradleRun{rule: "ComposeCompilerRemovedFlags", content: composeFlagsScript, kotlin: c.kotlin}.findings(t)
		got := sortedLines(fs)
		if len(got) != len(c.lines) {
			t.Errorf("kotlin %q: lines %v, want %v", c.kotlin, got, c.lines)
			continue
		}
		for i := range got {
			if got[i] != c.lines[i] {
				t.Errorf("kotlin %q: lines %v, want %v", c.kotlin, got, c.lines)
				break
			}
		}
		for _, f := range fs {
			strongFlagOnly := strings.Contains(f.Message, "OptimizeNonSkippingGroups") || strings.Contains(f.Message, "PausableComposition")
			if strongFlagOnly {
				if f.Severity != "warning" {
					t.Errorf("kotlin %q: deprecated-only flag severity = %q", c.kotlin, f.Severity)
				}
				continue
			}
			if f.Severity != c.severity || f.Confidence != c.conf {
				t.Errorf("kotlin %q: %q severity %q conf %v, want %q %v", c.kotlin, f.Message, f.Severity, f.Confidence, c.severity, c.conf)
			}
		}
	}
}

func TestComposeCompilerRemovedFlagsUnknownVersion(t *testing.T) {
	fs := kotlinGradleRun{rule: "ComposeCompilerRemovedFlags", content: composeFlagsScript}.findings(t)
	if got := sortedLines(fs); len(got) != 7 {
		t.Fatalf("lines = %v, want the 7 error-class constructs (the two deprecated-only flags need 2.4.0)", got)
	}
	for _, f := range fs {
		if f.Severity != "warning" || f.Confidence != api.ConfidenceMedium || !strings.Contains(f.Message, "not known") {
			t.Errorf("finding = %+v", f)
		}
	}
}

func TestComposeCompilerRemovedFlagsMessagesAndAnchors(t *testing.T) {
	fs := kotlinGradleRun{rule: "ComposeCompilerRemovedFlags", content: composeFlagsScript, kotlin: "2.4.20"}.findings(t)
	byToken := map[string]string{}
	for _, f := range fs {
		byToken[composeFlagsScript[f.StartByte:f.EndByte]] = f.Message
	}
	for token, want := range map[string]string{
		"enableStrongSkippingMode":                     "error since Kotlin 2.4.0",
		"stabilityConfigurationFile":                   "`stabilityConfigurationFiles`",
		"generateFunctionKeyMetaClasses":               "`featureFlags`",
		"enableNonSkippingGroupOptimization":           "`featureFlags`",
		"ComposeFeatureFlag.StrongSkipping":            "2.5.0",
		"ComposeFeatureFlag.IntrinsicRemember":         "2.5.0",
		"ComposeFeatureFlag.PausableComposition":       "removed in 2.6.0",
		"ComposeFeatureFlag.OptimizeNonSkippingGroups": "deprecated since Kotlin 2.4.0",
	} {
		if !strings.Contains(byToken[token], want) {
			t.Errorf("%s message = %q, want %q", token, byToken[token], want)
		}
	}
	if _, ok := byToken["stabilityConfigurationFiles"]; ok {
		t.Error("the plural replacement property must not be reported")
	}
	if fs[0].Line != 6 || fs[0].Col != 5 {
		t.Errorf("first anchor = %d:%d, want 6:5", fs[0].Line, fs[0].Col)
	}
}

func TestComposeCompilerRemovedFlagsFixDeletesRestatedDefaults(t *testing.T) {
	fs := kotlinGradleRun{rule: "ComposeCompilerRemovedFlags", content: composeFlagsScript, kotlin: "2.4.20"}.findings(t)
	fixed := applyAllByteFixes(t, composeFlagsScript, fs)
	if strings.Contains(fixed, "    enableStrongSkippingMode") || strings.Contains(fixed, "    enableIntrinsicRemember") {
		t.Errorf("redundant `= true` lines must go:\n%s", fixed)
	}
	for _, keep := range []string{"enableNonSkippingGroupOptimization", "stabilityConfigurationFile =", "generateFunctionKeyMetaClasses", "ComposeFeatureFlag.StrongSkipping.disabled()"} {
		if !strings.Contains(fixed, keep) {
			t.Errorf("%s needs a manual decision and must stay:\n%s", keep, fixed)
		}
	}
	disabled := "composeCompiler {\n    enableStrongSkippingMode = false\n}\n"
	got := kotlinGradleRun{rule: "ComposeCompilerRemovedFlags", content: disabled, kotlin: "2.4.20"}.findings(t)
	if len(got) != 1 || got[0].Fix != nil {
		t.Errorf("an opt-out is report-only: %+v", got)
	}
}

func TestComposeCompilerRemovedFlagsQualifiedAndGroovy(t *testing.T) {
	content := "composeCompiler.enableStrongSkippingMode.set(false)\ncomposeCompiler {\n    stabilityConfigurationFile = file('stability.conf')\n}\nstabilityConfigurationFile = 1\nfoo { enableIntrinsicRemember = true }\n"
	fs := kotlinGradleRun{rule: "ComposeCompilerRemovedFlags", path: "build.gradle", content: content, kotlin: "2.4.20"}.findings(t)
	if got := sortedLines(fs); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("lines = %v, want the qualified use and the block use only", got)
	}
}
