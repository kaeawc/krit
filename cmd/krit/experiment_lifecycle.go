package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
)

// experimentCatalogPath is the path, relative to the module root, of the file
// containing the knownDefinitions catalog. Rewriting is text-based so we don't
// need to re-indent other entries.
const experimentCatalogPath = "internal/experiment/experiment.go"

// listExperimentsLifecyclePlain renders the experiment catalog grouped by
// lifecycle status (promoted → experimental → deprecated), matching the style
// used by -list-rules.
func listExperimentsLifecyclePlain() string {
	defs := experiment.Definitions()

	var promoted, experimental, deprecated []experiment.Definition
	for _, def := range defs {
		switch def.Status {
		case experiment.StatusPromoted:
			promoted = append(promoted, def)
		case experiment.StatusDeprecated:
			deprecated = append(deprecated, def)
		default:
			experimental = append(experimental, def)
		}
	}

	sortDefs := func(xs []experiment.Definition) {
		sort.Slice(xs, func(i, j int) bool { return xs[i].Name < xs[j].Name })
	}
	sortDefs(promoted)
	sortDefs(experimental)
	sortDefs(deprecated)

	var b strings.Builder
	fmt.Fprintf(&b, "Experiments (%d total)\n\n", len(defs))

	writeGroup := func(label string, group []experiment.Definition) {
		fmt.Fprintf(&b, "%s (%d)\n", label, len(group))
		for _, def := range group {
			intent := def.Intent
			if intent == "" {
				intent = "unspecified"
			}
			targets := "-"
			if len(def.TargetRules) > 0 {
				targets = strings.Join(def.TargetRules, ", ")
			}
			fmt.Fprintf(&b, "  %-50s %-14s %s\n", def.Name, intent, targets)
			for _, line := range wrapIndented(def.Description, 76, "    ") {
				fmt.Fprintln(&b, line)
			}
			fmt.Fprintln(&b)
		}
		if len(group) == 0 {
			fmt.Fprintln(&b)
		}
	}

	writeGroup("PROMOTED", promoted)
	writeGroup("EXPERIMENTAL", experimental)
	writeGroup("DEPRECATED", deprecated)
	return b.String()
}

// wrapIndented splits s into lines no wider than width (best-effort, word-wrap)
// and prefixes each line with indent.
func wrapIndented(s string, width int, indent string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	words := strings.Fields(s)
	var lines []string
	var cur strings.Builder
	cur.WriteString(indent)
	curLen := 0
	for _, w := range words {
		if curLen == 0 {
			cur.WriteString(w)
			curLen = len(w)
			continue
		}
		if curLen+1+len(w) > width {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(indent)
			cur.WriteString(w)
			curLen = len(w)
			continue
		}
		cur.WriteByte(' ')
		cur.WriteString(w)
		curLen += 1 + len(w)
	}
	if curLen > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// promoteExperiment rewrites the Status field of the named experiment in
// internal/experiment/experiment.go to "promoted" (or "deprecated" if
// newStatus == StatusDeprecated). On success it writes the file back and runs
// `go build ./...`; if the build fails, the original file is restored.
func promoteExperiment(name, newStatus string) int {
	def, ok := experiment.Lookup(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: experiment %q not in catalog\n", name)
		return 2
	}
	switch newStatus {
	case experiment.StatusPromoted:
		switch def.Status {
		case experiment.StatusPromoted:
			fmt.Fprintf(os.Stderr, "error: experiment %q is already promoted\n", name)
			return 0
		case experiment.StatusDeprecated:
			fmt.Fprintf(os.Stderr, "error: experiment %q is deprecated, cannot promote\n", name)
			return 2
		}
	case experiment.StatusDeprecated:
		if def.Status == experiment.StatusDeprecated {
			fmt.Fprintf(os.Stderr, "error: experiment %q is already deprecated\n", name)
			return 0
		}
	default:
		fmt.Fprintf(os.Stderr, "error: invalid target status %q\n", newStatus)
		return 2
	}

	path := experimentCatalogPath
	orig, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", path, err)
		return 2
	}

	updated, err := rewriteExperimentStatus(string(orig), name, newStatus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if updated == string(orig) {
		fmt.Fprintf(os.Stderr, "error: no change applied for %q (source already matches target status)\n", name)
		return 2
	}

	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", path, err)
		return 2
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if wErr := os.WriteFile(path, orig, 0644); wErr != nil {
			fmt.Fprintf(os.Stderr, "error: build failed and could not restore %s: %v (original write error: %v)\n", path, wErr, err)
			return 3
		}
		fmt.Fprintf(os.Stderr, "error: go build failed after rewrite; restored %s: %v\n", path, err)
		return 3
	}

	switch newStatus {
	case experiment.StatusPromoted:
		fmt.Printf("Promoted experiment %q. It will now be enabled by default.\n", name)
	case experiment.StatusDeprecated:
		fmt.Printf("Deprecated experiment %q. It will no longer be enabled.\n", name)
	}
	return 0
}

// rewriteExperimentStatus performs the text-based struct-literal rewrite.
// It locates the literal whose Name: "<name>" matches, finds its enclosing
// braces, and either replaces the existing Status: "..." field value or
// inserts `, Status: "<newStatus>"` just before the closing brace.
func rewriteExperimentStatus(src, name, newStatus string) (string, error) {
	needle := fmt.Sprintf(`Name: %q`, name)
	idx := strings.Index(src, needle)
	if idx < 0 {
		return "", fmt.Errorf("could not locate %s in catalog source", needle)
	}

	// Find opening brace scanning backward.
	open := -1
	for i := idx; i >= 0; i-- {
		if src[i] == '{' {
			open = i
			break
		}
		if src[i] == '}' {
			return "", fmt.Errorf("unexpected '}' before %s", needle)
		}
	}
	if open < 0 {
		return "", fmt.Errorf("could not find opening '{' before %s", needle)
	}

	// Find matching closing brace scanning forward. The existing catalog uses
	// flat literals per entry (no nested braces inside {...} except possibly
	// the string slice []string{...}), so we track depth.
	depth := 0
	close := -1
	inString := false
	var strQuote byte
	for i := open; i < len(src); i++ {
		c := src[i]
		if inString {
			if c == '\\' && i+1 < len(src) {
				i++
				continue
			}
			if c == strQuote {
				inString = false
			}
			continue
		}
		switch c {
		case '"', '`':
			inString = true
			strQuote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				close = i
			}
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 {
		return "", fmt.Errorf("could not find closing '}' after %s", needle)
	}

	body := src[open+1 : close]

	// Look for an existing Status: "..." field in this literal.
	statusKey := "Status:"
	if sIdx := strings.Index(body, statusKey); sIdx >= 0 {
		// Find the opening quote of the value.
		rest := body[sIdx+len(statusKey):]
		q1 := strings.IndexByte(rest, '"')
		if q1 < 0 {
			return "", fmt.Errorf("malformed Status field in %s literal", name)
		}
		q2 := strings.IndexByte(rest[q1+1:], '"')
		if q2 < 0 {
			return "", fmt.Errorf("malformed Status value in %s literal", name)
		}
		// Replace the value between the quotes.
		absStart := open + 1 + sIdx + len(statusKey) + q1 + 1
		absEnd := absStart + q2
		return src[:absStart] + newStatus + src[absEnd:], nil
	}

	// No Status field — insert `, Status: "<newStatus>"` just before the
	// closing brace, preserving any trailing whitespace.
	trimmedBody := strings.TrimRight(body, " \t")
	insertAt := open + 1 + len(trimmedBody)
	insertion := fmt.Sprintf(`, Status: %q`, newStatus)
	return src[:insertAt] + insertion + src[insertAt:], nil
}
