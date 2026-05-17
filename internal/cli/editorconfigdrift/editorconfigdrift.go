package editorconfigdrift

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/scanner"
)

// Run implements `krit editorconfig-drift [paths...]`.
func Run(args []string) int {
	paths := args
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := scanner.CollectKotlinFiles(paths, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "editorconfig-drift: collecting files: %v\n", err)
		return 1
	}
	sort.Strings(files)

	summary := newSummary()
	for _, path := range files {
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "editorconfig-drift: parsing %s: %v\n", path, err)
			continue
		}
		summary.add(file, config.LoadEditorConfig(file.Path))
	}

	lines := summary.lines()
	if len(lines) == 0 {
		fmt.Fprintln(os.Stdout, "No editorconfig drift found.")
		return 0
	}
	for _, line := range lines {
		fmt.Fprintln(os.Stdout, line)
	}
	return 0
}

type driftSummary struct {
	indentSize      map[int]map[int]int
	indentTabs      int
	indentSpaces    int
	longLineFiles   map[int]int
	longLineCounts  map[int]int
	invalidUTF8     int
	missingFinalNL  int
	extraFinalNL    int
	trailingFiles   int
	trailingLineCnt int
}

func newSummary() *driftSummary {
	return &driftSummary{
		indentSize:     make(map[int]map[int]int),
		longLineFiles:  make(map[int]int),
		longLineCounts: make(map[int]int),
	}
}

func (s *driftSummary) add(file *scanner.File, ec *config.EditorConfig) {
	if file == nil || ec == nil {
		return
	}
	s.addIndentDrift(file.Lines, ec)
	s.addLongLineDrift(file.Lines, ec)
	s.addCharsetDrift(file.Content, ec)
	s.addFinalNewlineDrift(file.Content, ec)
	s.addTrailingWhitespaceDrift(file.Lines, ec)
}

func (s *driftSummary) addIndentDrift(lines []string, ec *config.EditorConfig) {
	if ec.IndentSize > 0 {
		if observed := observedSpaceIndent(lines); observed > 0 && observed != ec.IndentSize {
			if s.indentSize[ec.IndentSize] == nil {
				s.indentSize[ec.IndentSize] = make(map[int]int)
			}
			s.indentSize[ec.IndentSize][observed]++
		}
	}
	if ec.IndentStyle == "space" && fileHasLeadingTabs(lines) {
		s.indentTabs++
	}
	if ec.IndentStyle == "tab" && fileHasLeadingSpaces(lines) {
		s.indentSpaces++
	}
}

func (s *driftSummary) addLongLineDrift(lines []string, ec *config.EditorConfig) {
	if ec.MaxLineLength <= 0 {
		return
	}
	overflow := countLongLines(lines, ec.MaxLineLength)
	if overflow > 0 {
		s.longLineFiles[ec.MaxLineLength]++
		s.longLineCounts[ec.MaxLineLength] += overflow
	}
}

func (s *driftSummary) addCharsetDrift(content []byte, ec *config.EditorConfig) {
	if ec.Charset == "utf-8" && !utf8.Valid(content) {
		s.invalidUTF8++
	}
}

func (s *driftSummary) addFinalNewlineDrift(content []byte, ec *config.EditorConfig) {
	if ec.InsertFinalNewline == nil {
		return
	}
	hasFinal := len(content) > 0 && content[len(content)-1] == '\n'
	if *ec.InsertFinalNewline && !hasFinal {
		s.missingFinalNL++
	}
	if !*ec.InsertFinalNewline && hasFinal {
		s.extraFinalNL++
	}
}

func (s *driftSummary) addTrailingWhitespaceDrift(lines []string, ec *config.EditorConfig) {
	if ec.TrimTrailingWhitespace == nil || !*ec.TrimTrailingWhitespace {
		return
	}
	if n := countTrailingWhitespaceLines(lines); n > 0 {
		s.trailingFiles++
		s.trailingLineCnt += n
	}
}

func (s *driftSummary) lines() []string {
	var lines []string
	for _, configured := range sortedNestedIntKeys(s.indentSize) {
		for _, observed := range sortedInts(s.indentSize[configured]) {
			lines = append(lines, fmt.Sprintf(".editorconfig says indent_size=%d; %d files use %d-space indentation.", configured, s.indentSize[configured][observed], observed))
		}
	}
	if s.indentTabs > 0 {
		lines = append(lines, fmt.Sprintf(".editorconfig says indent_style=space; %d files use tab indentation.", s.indentTabs))
	}
	if s.indentSpaces > 0 {
		lines = append(lines, fmt.Sprintf(".editorconfig says indent_style=tab; %d files use space indentation.", s.indentSpaces))
	}
	for _, max := range sortedInts(s.longLineFiles) {
		lines = append(lines, fmt.Sprintf(".editorconfig says max_line_length=%d; %d files exceed it on %d lines.", max, s.longLineFiles[max], s.longLineCounts[max]))
	}
	if s.invalidUTF8 > 0 {
		lines = append(lines, fmt.Sprintf(".editorconfig says charset=utf-8; %d files are not valid UTF-8.", s.invalidUTF8))
	}
	if s.missingFinalNL > 0 {
		lines = append(lines, fmt.Sprintf(".editorconfig says insert_final_newline=true; %d files are missing a final newline.", s.missingFinalNL))
	}
	if s.extraFinalNL > 0 {
		lines = append(lines, fmt.Sprintf(".editorconfig says insert_final_newline=false; %d files end with a final newline.", s.extraFinalNL))
	}
	if s.trailingFiles > 0 {
		lines = append(lines, fmt.Sprintf(".editorconfig says trim_trailing_whitespace=true; %d files contain trailing whitespace on %d lines.", s.trailingFiles, s.trailingLineCnt))
	}
	return lines
}

func observedSpaceIndent(lines []string) int {
	best := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "\t") {
			continue
		}
		n := 0
		for n < len(line) && line[n] == ' ' {
			n++
		}
		if n == 0 {
			continue
		}
		if best == 0 || n < best {
			best = n
		}
	}
	return best
}

func fileHasLeadingTabs(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			return true
		}
	}
	return false
}

func fileHasLeadingSpaces(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, " ") {
			return true
		}
	}
	return false
}

func countLongLines(lines []string, maxLen int) int {
	count := 0
	for _, line := range lines {
		if len(line) > maxLen {
			count++
		}
	}
	return count
}

func countTrailingWhitespaceLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			count++
		}
	}
	return count
}

func sortedInts(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func sortedNestedIntKeys(m map[int]map[int]int) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}
