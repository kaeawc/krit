package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// isTerminal returns true if the writer is a terminal and colors are allowed.
// Respects NO_COLOR env var (https://no-color.org).
func isTerminal(w io.Writer) bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return fi.Mode()&os.ModeCharDevice != 0
	}
	return false
}

func normalizedFindingColumns(columns *scanner.FindingColumns) scanner.FindingColumns {
	if columns == nil {
		return scanner.FindingColumns{}
	}
	return *columns
}

// FormatPlain writes findings as plain text with optional color.
func FormatPlain(w io.Writer, findings []scanner.Finding) {
	columns := scanner.CollectFindings(findings)
	FormatPlainColumns(w, &columns)
}

// FormatPlainColumns writes columnar findings as plain text with optional color.
func FormatPlainColumns(w io.Writer, columns *scanner.FindingColumns) {
	color := isTerminal(w)
	normalized := normalizedFindingColumns(columns)
	normalized.VisitSortedByFileLine(func(row int) {
		file := normalized.FileAt(row)
		line := normalized.LineAt(row)
		col := normalized.ColumnAt(row)
		severity := normalized.SeverityAt(row)
		ruleSet := normalized.RuleSetAt(row)
		rule := normalized.RuleAt(row)
		message := normalized.MessageAt(row)
		confidence := normalized.ConfidenceAt(row)
		if color {
			sevColor := colorYellow
			if severity == "error" {
				sevColor = colorRed
			}
			fmt.Fprintf(w, "%s%s:%d:%d%s: %s%s%s %s[%s:%s]%s %s\n",
				colorBold, file, line, col, colorReset,
				sevColor, severity, colorReset,
				colorCyan, ruleSet, rule, colorReset,
				message)
		} else if confidence > 0 {
			fmt.Fprintf(w, "%s:%d:%d: %s [%.2f]: [%s:%s] %s\n",
				file, line, col, severity, confidence, ruleSet, rule, message)
		} else {
			fmt.Fprintf(w, "%s:%d:%d: %s: [%s:%s] %s\n",
				file, line, col, severity, ruleSet, rule, message)
		}
	})
}

// SARIF types for proper JSON marshaling.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Rules   []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string     `json:"id"`
	ShortDescription sarifText  `json:"shortDescription"`
	FullDescription  *sarifText `json:"fullDescription,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID     string           `json:"ruleId"`
	Level      string           `json:"level"`
	Message    sarifText        `json:"message"`
	Locations  []sarifLocation  `json:"locations"`
	Properties *sarifProperties `json:"properties,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
}

type sarifProperties struct {
	Confidence float64 `json:"confidence"`
}

// FormatSARIF writes findings as SARIF 2.1.0 JSON.
func FormatSARIF(w io.Writer, findings []scanner.Finding, version string) error {
	columns := scanner.CollectFindings(findings)
	return FormatSARIFColumns(w, &columns, version)
}

// FormatSARIFColumns writes columnar findings as SARIF 2.1.0 JSON.
func FormatSARIFColumns(w io.Writer, columns *scanner.FindingColumns, version string) error {
	cols := normalizedFindingColumns(columns)

	// Build description map from rule registry.
	descMap := make(map[string]string)
	for _, r := range v2.Registry {
		key := r.Category + "/" + r.ID
		if r.Description != "" {
			descMap[key] = r.Description
		}
	}

	rulesSeen := make(map[string]bool)
	var sarifRules []sarifRule

	var results []sarifResult
	cols.VisitSortedByFileLine(func(row int) {
		ruleID := cols.RuleSetAt(row) + "/" + cols.RuleAt(row)
		if !rulesSeen[ruleID] {
			rulesSeen[ruleID] = true
			sr := sarifRule{
				ID:               ruleID,
				ShortDescription: sarifText{Text: ruleID},
			}
			if desc, ok := descMap[ruleID]; ok {
				sr.FullDescription = &sarifText{Text: desc}
			}
			sarifRules = append(sarifRules, sr)
		}

		severity := cols.SeverityAt(row)
		level := "note"
		if severity == "error" {
			level = "error"
		} else if severity == "warning" {
			level = "warning"
		}
		r := sarifResult{
			RuleID:  ruleID,
			Level:   level,
			Message: sarifText{Text: cols.MessageAt(row)},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: cols.FileAt(row)},
					Region:           sarifRegion{StartLine: cols.LineAt(row), StartColumn: cols.ColumnAt(row)},
				},
			}},
		}
		if confidence := cols.ConfidenceAt(row); confidence > 0 {
			r.Properties = &sarifProperties{Confidence: confidence}
		}
		results = append(results, r)
	})

	log := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:    "krit",
					Version: version,
					Rules:   sarifRules,
				},
			},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(log)
}

// FormatCheckstyle writes findings as Checkstyle XML.
func FormatCheckstyle(w io.Writer, findings []scanner.Finding) {
	columns := scanner.CollectFindings(findings)
	FormatCheckstyleColumns(w, &columns)
}

// FormatCheckstyleColumns writes columnar findings as Checkstyle XML.
func FormatCheckstyleColumns(w io.Writer, columns *scanner.FindingColumns) {
	normalized := normalizedFindingColumns(columns)

	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(w, `<checkstyle version="8.0">`)

	currentFile := ""
	normalized.VisitSortedByFileLine(func(row int) {
		file := normalized.FileAt(row)
		if file != currentFile {
			if currentFile != "" {
				fmt.Fprintln(w, `  </file>`)
			}
			currentFile = file
			fmt.Fprintf(w, "  <file name=\"%s\">\n", xmlEscape(currentFile))
		}
		if confidence := normalized.ConfidenceAt(row); confidence > 0 {
			fmt.Fprintf(w, "    <error line=\"%d\" column=\"%d\" severity=\"%s\" message=\"%s\" source=\"%s.%s\" confidence=\"%.2f\"/>\n",
				normalized.LineAt(row), normalized.ColumnAt(row), normalized.SeverityAt(row), xmlEscape(normalized.MessageAt(row)),
				normalized.RuleSetAt(row), normalized.RuleAt(row), confidence)
		} else {
			fmt.Fprintf(w, "    <error line=\"%d\" column=\"%d\" severity=\"%s\" message=\"%s\" source=\"%s.%s\"/>\n",
				normalized.LineAt(row), normalized.ColumnAt(row), normalized.SeverityAt(row), xmlEscape(normalized.MessageAt(row)),
				normalized.RuleSetAt(row), normalized.RuleAt(row))
		}
	})
	if currentFile != "" {
		fmt.Fprintln(w, `  </file>`)
	}
	fmt.Fprintln(w, `</checkstyle>`)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
