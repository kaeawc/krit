package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// indentationRules report a position inside leading whitespace on purpose.
var indentationRules = map[string]bool{
	"Indentation":                true,
	"NoTabs":                     true,
	"MaximumLineLength":          true,
	"TrailingWhitespace":         true,
	"NoMultipleSpaces":           true,
	"ArgumentListWrapping":       true,
	"ParameterListWrapping":      true,
	"MultiLineIfElse":            true,
	"WrongIndentation":           true,
	"NewLineAtEndOfFile":         true,
	"NoConsecutiveBlankLines":    true,
	"NoBlankLineBeforeRbrace":    true,
	"NoBlankLineAfterLbrace":     true,
	"NoEmptyFirstLineInMethod":   true,
	"SpacingAroundCurly":         true,
	"SpacingAroundOperators":     true,
	"SpacingAroundParens":        true,
	"SpacingAroundComma":         true,
	"SpacingAroundKeyword":       true,
	"SpacingAroundColon":         true,
	"SpacingAroundDot":           true,
	"SpacingAroundRangeOperator": true,
}

// TestFixtureFindingPositions runs every single-file rule over the positive
// fixtures and checks each finding sits on a real column of its line and does
// not point into leading whitespace (a sign of a line-only emitter).
func TestFixtureFindingPositions(t *testing.T) {
	root := filepath.Join(fixtureRoot(t), "positive")
	var single []*api.Rule
	for _, r := range api.Registry {
		if r.Needs.Has(api.NeedsCrossFile) || r.Needs.Has(api.NeedsModuleIndex) ||
			r.Needs.Has(api.NeedsParsedFiles) || r.Needs.Has(api.NeedsResources) ||
			r.Needs.Has(api.NeedsManifest) || r.Needs.Has(api.NeedsGradle) {
			continue
		}
		single = append(single, r)
	}
	bad := map[string]string{}
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".kt") {
			return err
		}
		file, perr := scanner.ParseFile(context.Background(), path)
		if perr != nil {
			return nil
		}
		count++
		d := rules.NewDispatcher(single, nil)
		cols := d.Run(file)
		for _, f := range cols.Findings() {
			if reason := positionProblem(file, f); reason != "" && !indentationRules[f.Rule] {
				if _, seen := bad[f.Rule]; !seen {
					bad[f.Rule] = reason + " at " + filepath.Base(path) + ":" + strconv.Itoa(f.Line) + ":" + strconv.Itoa(f.Col)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no positive fixtures found")
	}
	for rule, why := range bad {
		t.Errorf("%s: %s", rule, why)
	}
}

func positionProblem(file *scanner.File, f scanner.Finding) string {
	if f.Line < 1 || f.Line > len(file.Lines) {
		return "line out of range"
	}
	if f.Col < 1 {
		return "column < 1"
	}
	line := file.Lines[f.Line-1]
	if f.Col > len(line)+1 {
		return "column beyond end of line"
	}
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	if strings.TrimSpace(line) != "" && f.Col-1 < indent {
		return "column points into leading whitespace"
	}
	return ""
}
