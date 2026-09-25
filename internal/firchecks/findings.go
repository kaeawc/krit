package firchecks

import (
	"sync"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// FirFinding is the per-finding JSON shape emitted by krit-fir.
type FirFinding struct {
	Path       string  `json:"path"`
	Line       int     `json:"line"`
	Col        int     `json:"col"`
	StartByte  int     `json:"startByte,omitempty"`
	EndByte    int     `json:"endByte,omitempty"`
	Rule       string  `json:"rule"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	Confidence float64 `json:"confidence"`
}

// CheckResponse is the JSON envelope returned by krit-fir for a "check" request.
type CheckResponse struct {
	ID        int64             `json:"id"`
	Succeeded int               `json:"succeeded"`
	Skipped   int               `json:"skipped"`
	Findings  []FirFinding      `json:"findings"`
	Crashed   map[string]string `json:"crashed"`
	Rules     []string          `json:"rules"`
	// ErrorFiles maps each requested file whose checker verdict is not
	// authoritative to the first reason: an ERROR-severity compiler
	// diagnostic in (or affecting) the file, or the file not being part of
	// the JVM compilation. Go keeps its own findings for these files.
	ErrorFiles map[string]string `json:"errorFiles"`
	// RuleErrors maps rule ID -> requested file -> the exception that rule's
	// checker threw on the file. krit-fir isolates a checker exception to its
	// rule: the compile and every other rule continue, and only that
	// (file, rule) verdict is not authoritative.
	RuleErrors map[string]map[string]string `json:"ruleErrors"`
}

var catalogOnce sync.Once
var catalogByID map[string]*api.Rule

func catalogRule(id string) *api.Rule {
	catalogOnce.Do(func() {
		catalogByID = make(map[string]*api.Rule, len(api.Registry))
		for _, rule := range api.Registry {
			if rule != nil {
				catalogByID[rule.ID] = rule
			}
		}
	})
	return catalogByID[id]
}

// ToScannerFinding preserves the identity rule ID and resolves catalog metadata.
func ToScannerFinding(f FirFinding) scanner.Finding {
	sev := f.Severity
	if sev == "" {
		sev = "warning"
	}
	ruleSet := "fir"
	if rule := catalogRule(f.Rule); rule != nil {
		ruleSet = rule.Category
		if rule.Sev != "" {
			sev = string(rule.Sev)
		}
	}
	return scanner.Finding{
		File:       f.Path,
		Line:       f.Line,
		Col:        f.Col,
		StartByte:  f.StartByte,
		EndByte:    f.EndByte,
		RuleSet:    ruleSet,
		Rule:       f.Rule,
		Severity:   sev,
		Message:    f.Message,
		Confidence: f.Confidence,
	}
}
