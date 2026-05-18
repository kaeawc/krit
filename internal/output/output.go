package output

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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
	Tool       sarifTool       `json:"tool"`
	Results    []sarifResult   `json:"results"`
	Taxonomies []sarifTaxonomy `json:"taxonomies,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name                string                   `json:"name"`
	Version             string                   `json:"version"`
	Rules               []sarifRule              `json:"rules"`
	SupportedTaxonomies []sarifTaxonomyReference `json:"supportedTaxonomies,omitempty"`
}

type sarifRule struct {
	ID               string               `json:"id"`
	ShortDescription sarifText            `json:"shortDescription"`
	FullDescription  *sarifText           `json:"fullDescription,omitempty"`
	HelpURI          string               `json:"helpUri,omitempty"`
	Properties       *sarifRuleProperties `json:"properties,omitempty"`
	Relationships    []sarifRelationship  `json:"relationships,omitempty"`
}

type sarifRuleProperties struct {
	Maturity  string `json:"maturity,omitempty"`
	Precision string `json:"precision,omitempty"`
	Cost      string `json:"cost,omitempty"`
}

type sarifTaxonomy struct {
	Name             string       `json:"name"`
	ShortDescription sarifText    `json:"shortDescription"`
	Taxa             []sarifTaxon `json:"taxa"`
}

type sarifTaxon struct {
	ID               string    `json:"id"`
	ShortDescription sarifText `json:"shortDescription"`
}

type sarifTaxonomyReference struct {
	Name string `json:"name"`
}

type sarifRelationship struct {
	Target sarifReportingDescriptorReference `json:"target"`
	Kinds  []string                          `json:"kinds"`
}

type sarifReportingDescriptorReference struct {
	ID            string                `json:"id"`
	ToolComponent sarifToolComponentRef `json:"toolComponent"`
}

type sarifToolComponentRef struct {
	Name string `json:"name"`
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
	Confidence   float64  `json:"confidence,omitempty"`
	Precision    string   `json:"precision,omitempty"`
	Effort       string   `json:"effort,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	CWE          []string `json:"cwe,omitempty"`
	OWASP        []string `json:"owasp,omitempty"`
	SEICert      []string `json:"sei-cert,omitempty"`
	Mitre        []string `json:"mitre,omitempty"`
}

// FormatSARIFColumns writes columnar findings as SARIF 2.1.0 JSON.
func FormatSARIFColumns(w io.Writer, columns *scanner.FindingColumns, version string) error {
	cols := normalizedFindingColumns(columns)

	descMap := make(map[string]string)
	helpURIMap := make(map[string]string)
	precisionMap := make(map[string]string)
	effortMap := make(map[string]string)
	costMap := make(map[string]string)
	capabilityMap := make(map[string][]string)
	maturityMap := make(map[string]string)
	securityMap := make(map[string]*api.SecurityTaxonomy)
	for _, r := range api.Registry {
		key := r.Category + "/" + r.ID
		if r.Description != "" {
			descMap[key] = r.Description
		}
		if uri := api.RuleDocsURL(r); uri != "" {
			helpURIMap[key] = uri
		}
		precisionMap[key] = rules.V2RulePrecision(r).String()
		effortMap[key] = rules.V2RuleEffort(r).String()
		costMap[key] = rules.CostFor(r).String()
		if list := r.CapabilitiesList(); len(list) > 0 {
			capabilityMap[key] = list
		}
		maturityMap[key] = r.Maturity.String()
		if r.Security != nil && !r.Security.IsEmpty() {
			securityMap[key] = r.Security
		}
	}

	rulesSeen := make(map[string]bool)
	var sarifRules []sarifRule
	cweIDs := newOrderedTaxa()
	owaspIDs := newOrderedTaxa()
	certIDs := newOrderedTaxa()
	mitreIDs := newOrderedTaxa()

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
			if uri, ok := helpURIMap[ruleID]; ok {
				sr.HelpURI = uri
			}
			maturity := maturityMap[ruleID]
			precision := precisionMap[ruleID]
			cost := costMap[ruleID]
			if cost == "unset" {
				cost = ""
			}
			if maturity != "" || precision != "" || cost != "" {
				sr.Properties = &sarifRuleProperties{Maturity: maturity, Precision: precision, Cost: cost}
			}
			if sec := securityMap[ruleID]; sec != nil {
				sr.Relationships = sarifTaxonomyRelationships(sec, cweIDs, owaspIDs, certIDs, mitreIDs)
			}
			sarifRules = append(sarifRules, sr)
		}

		severity := cols.SeverityAt(row)
		level := "note"
		switch severity {
		case "error":
			level = "error"
		case "warning":
			level = "warning"
		}
		r := sarifResult{
			RuleID:  ruleID,
			Level:   level,
			Message: sarifText{Text: cols.MessageAt(row)},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: pathToSARIFURI(cols.FileAt(row))},
					Region:           sarifRegion{StartLine: cols.LineAt(row), StartColumn: cols.ColumnAt(row)},
				},
			}},
		}
		r.Properties = buildResultProperties(cols.ConfidenceAt(row), precisionMap[ruleID], effortMap[ruleID], capabilityMap[ruleID], securityMap[ruleID])
		results = append(results, r)
	})

	taxonomies, supported := buildSarifTaxonomies(cweIDs, owaspIDs, certIDs, mitreIDs)

	log := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:                "krit",
					Version:             version,
					Rules:               sarifRules,
					SupportedTaxonomies: supported,
				},
			},
			Results:    results,
			Taxonomies: taxonomies,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(log)
}

// buildResultProperties returns the per-finding sarifProperties payload,
// or nil when the finding has no metadata worth emitting.
func buildResultProperties(confidence float64, precision, effort string, caps []string, sec *api.SecurityTaxonomy) *sarifProperties {
	if confidence <= 0 && precision == "" && effort == "" && len(caps) == 0 && sec == nil {
		return nil
	}
	props := &sarifProperties{Confidence: confidence, Precision: precision, Effort: effort, Capabilities: caps}
	if sec != nil {
		props.CWE = sec.CWE
		props.OWASP = sec.OWASP
		props.SEICert = sec.SEICert
		props.Mitre = sec.Mitre
	}
	return props
}

// orderedTaxa is an insertion-ordered set used while collecting
// SARIF taxa. Insertion order yields stable output without sorting.
type orderedTaxa struct {
	seen  map[string]struct{}
	order []string
}

func newOrderedTaxa() *orderedTaxa {
	return &orderedTaxa{seen: map[string]struct{}{}}
}

func (o *orderedTaxa) add(id string) {
	if id == "" {
		return
	}
	if _, ok := o.seen[id]; ok {
		return
	}
	o.seen[id] = struct{}{}
	o.order = append(o.order, id)
}

func (o *orderedTaxa) values() []string { return o.order }

// SARIF taxonomy names. Consumers (GitHub code-scanning, Snyk) key
// off these names to link findings to the public catalogs.
const (
	taxonomyCWE     = "CWE"
	taxonomyOWASP   = "OWASP"
	taxonomySEICert = "SEI-CERT"
	taxonomyMitre   = "MITRE-ATT&CK"
)

func sarifTaxonomyRelationships(sec *api.SecurityTaxonomy, cwe, owasp, cert, mitre *orderedTaxa) []sarifRelationship {
	if sec == nil {
		return nil
	}
	var rels []sarifRelationship
	add := func(ids []string, taxonomy string, sink *orderedTaxa) {
		for _, id := range ids {
			sink.add(id)
			rels = append(rels, sarifRelationship{
				Target: sarifReportingDescriptorReference{
					ID:            id,
					ToolComponent: sarifToolComponentRef{Name: taxonomy},
				},
				Kinds: []string{"superset"},
			})
		}
	}
	add(sec.CWE, taxonomyCWE, cwe)
	add(sec.OWASP, taxonomyOWASP, owasp)
	add(sec.SEICert, taxonomySEICert, cert)
	add(sec.Mitre, taxonomyMitre, mitre)
	return rels
}

func buildSarifTaxonomies(cwe, owasp, cert, mitre *orderedTaxa) ([]sarifTaxonomy, []sarifTaxonomyReference) {
	type bucket struct {
		name string
		desc string
		ids  []string
	}
	buckets := []bucket{
		{taxonomyCWE, "Common Weakness Enumeration", cwe.values()},
		{taxonomyOWASP, "OWASP Top 10", owasp.values()},
		{taxonomySEICert, "SEI CERT Secure Coding", cert.values()},
		{taxonomyMitre, "MITRE ATT&CK", mitre.values()},
	}
	var taxonomies []sarifTaxonomy
	var supported []sarifTaxonomyReference
	for _, b := range buckets {
		if len(b.ids) == 0 {
			continue
		}
		taxa := make([]sarifTaxon, 0, len(b.ids))
		for _, id := range b.ids {
			taxa = append(taxa, sarifTaxon{ID: id, ShortDescription: sarifText{Text: id}})
		}
		taxonomies = append(taxonomies, sarifTaxonomy{
			Name:             b.name,
			ShortDescription: sarifText{Text: b.desc},
			Taxa:             taxa,
		})
		supported = append(supported, sarifTaxonomyReference{Name: b.name})
	}
	return taxonomies, supported
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

// pathToSARIFURI converts a filesystem path into a SARIF-safe
// artifactLocation.uri reference. SARIF 2.1.0 requires the value to be a
// valid URI reference per RFC 3986, so spaces, '#', '?', other reserved
// characters, and non-ASCII scalars must be percent-encoded. Windows
// backslashes are normalised to forward slashes.
//
// Encoding rules:
//   - Empty input returns "" unchanged.
//   - Absolute POSIX paths ("/foo/bar") become "file:///foo/bar" with each
//     path segment percent-encoded.
//   - Windows drive paths ("C:\\foo\\bar") become "file:///C:/foo/bar".
//   - Windows UNC paths ("\\\\srv\\share\\foo") become
//     "file://srv/share/foo".
//   - Relative paths stay relative (no file:// scheme is prepended) so that
//     SARIF consumers can resolve them against their own uriBaseId — but
//     reserved characters in each path segment are still percent-encoded.
//
// Slashes between segments are preserved verbatim; url.PathEscape is applied
// per segment to keep them as path separators rather than encoding them.
func pathToSARIFURI(path string) string {
	if path == "" {
		return ""
	}
	// UNC path: \\server\share\rest
	if strings.HasPrefix(path, `\\`) {
		rest := strings.TrimPrefix(path, `\\`)
		rest = strings.ReplaceAll(rest, `\`, "/")
		parts := strings.SplitN(rest, "/", 2)
		host := parts[0]
		tail := ""
		if len(parts) == 2 {
			tail = parts[1]
		}
		return "file://" + encodeURIHost(host) + "/" + encodeURIPath(tail)
	}
	// Windows drive: C:\foo or C:/foo
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		drive := string(path[0]) + ":"
		rest := strings.TrimLeft(path[2:], `\/`)
		rest = strings.ReplaceAll(rest, `\`, "/")
		return "file:///" + drive + "/" + encodeURIPath(rest)
	}
	// POSIX absolute path.
	if strings.HasPrefix(path, "/") {
		rest := strings.TrimPrefix(path, "/")
		return "file:///" + encodeURIPath(rest)
	}
	// Relative path: keep relative, normalise backslashes, encode segments.
	normalized := strings.ReplaceAll(path, `\`, "/")
	return encodeURIPath(normalized)
}

// encodeURIPath percent-encodes each '/'-separated segment with
// url.PathEscape while preserving the slashes themselves.
func encodeURIPath(p string) string {
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// encodeURIHost percent-encodes a host component for use in a file:// URI
// authority (UNC hosts). url.PathEscape is conservative enough — '/' and
// '?' would terminate the host but neither appears after our SplitN.
func encodeURIHost(h string) string {
	return url.PathEscape(h)
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
