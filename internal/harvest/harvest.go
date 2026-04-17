package harvest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

type Target struct {
	Path string
	Line int
}

type Result struct {
	Finding   scanner.Finding
	NodeType  string
	StartLine int
	EndLine   int
	Content   []byte
}

func ParseTarget(spec string) (Target, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Target{}, fmt.Errorf("empty target")
	}

	idx := strings.LastIndex(spec, ":")
	if idx <= 0 || idx == len(spec)-1 {
		return Target{}, fmt.Errorf("expected SOURCE:LINE, got %q", spec)
	}

	line, err := strconv.Atoi(spec[idx+1:])
	if err != nil || line < 1 {
		return Target{}, fmt.Errorf("invalid line in %q", spec)
	}

	return Target{
		Path: spec[:idx],
		Line: line,
	}, nil
}

func ExtractFixture(target Target, ruleName string) (Result, error) {
	rule, err := lookupRule(ruleName)
	if err != nil {
		return Result{}, err
	}
	if _, ok := rule.(interface {
		CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
	}); ok {
		return Result{}, fmt.Errorf("rule %s requires cross-file analysis and cannot be harvested from a single source file yet", ruleName)
	}
	if _, ok := rule.(interface {
		CheckModuleAware() []scanner.Finding
	}); ok {
		return Result{}, fmt.Errorf("rule %s requires module-aware analysis and cannot be harvested from a single source file yet", ruleName)
	}

	file, err := scanner.ParseFile(target.Path)
	if err != nil {
		return Result{}, err
	}

	findings := rules.NewDispatcher([]rules.Rule{rule}).Run(file)
	match, err := selectFinding(findings, target.Line, ruleName)
	if err != nil {
		return Result{}, err
	}

	col := match.Col
	if col < 1 {
		col = 1
	}
	offset := file.LineOffset(match.Line-1) + (col - 1)
	node, ok := file.FlatNamedDescendantForByteRange(uint32(offset), uint32(offset))
	if !ok || node == 0 {
		return Result{}, fmt.Errorf("no named AST node found at %s:%d:%d", target.Path, match.Line, col)
	}

	content := append([]byte(nil), file.FlatNodeBytes(node)...)
	if len(content) == 0 {
		return Result{}, fmt.Errorf("empty AST extraction at %s:%d:%d", target.Path, match.Line, col)
	}

	return Result{
		Finding:   match,
		NodeType:  file.FlatType(node),
		StartLine: file.FlatRow(node) + 1,
		EndLine:   lineForByteOffset(file, int(file.FlatEndByte(node)-1)),
		Content:   content,
	}, nil
}

func lineForByteOffset(file *scanner.File, offset int) int {
	if file == nil {
		return 1
	}
	offsets := file.LineOffsets()
	line := 0
	for i, start := range offsets {
		if start > offset {
			break
		}
		line = i
	}
	return line + 1
}

func WriteFixture(outPath string, result Result) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	content := append([]byte(nil), result.Content...)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}

	return os.WriteFile(outPath, content, 0644)
}

func lookupRule(ruleName string) (rules.Rule, error) {
	for _, rule := range rules.Registry {
		if rule.Name() == ruleName {
			return rule, nil
		}
	}
	return nil, fmt.Errorf("unknown rule %q", ruleName)
}

func selectFinding(findings []scanner.Finding, line int, ruleName string) (scanner.Finding, error) {
	var matches []scanner.Finding
	for _, finding := range findings {
		if finding.Rule == ruleName && finding.Line == line {
			matches = append(matches, finding)
		}
	}

	if len(matches) == 0 {
		return scanner.Finding{}, fmt.Errorf("rule %s reported no finding on line %d", ruleName, line)
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Col == matches[j].Col {
			return matches[i].Message < matches[j].Message
		}
		return matches[i].Col < matches[j].Col
	})

	if len(matches) > 1 {
		cols := make([]string, 0, len(matches))
		for _, finding := range matches {
			cols = append(cols, strconv.Itoa(finding.Col))
		}
		return scanner.Finding{}, fmt.Errorf("rule %s reported %d findings on line %d (cols %s); column-aware harvest is not wired yet",
			ruleName, len(matches), line, strings.Join(cols, ", "))
	}

	return matches[0], nil
}
