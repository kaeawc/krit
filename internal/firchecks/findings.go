package firchecks

import "github.com/kaeawc/krit/internal/scanner"

var firDiagnosticRuleNames = map[string]string{
	"FLOW_COLLECT_IN_ON_CREATE":    "CollectInOnCreateWithoutLifecycle",
	"COMPOSE_REMEMBER_WITHOUT_KEY": "ComposeRememberWithoutKey",
	"INJECT_DISPATCHER":            "InjectDispatcher",
	"UNSAFE_CAST_WHEN_NULLABLE":    "UnsafeCastWhenNullable",
	"SMOKE_CLASS":                  "SmokeChecker",
}

var firDiagnosticRuleSets = map[string]string{
	"FLOW_COLLECT_IN_ON_CREATE":    "coroutines",
	"COMPOSE_REMEMBER_WITHOUT_KEY": "compose",
	"INJECT_DISPATCHER":            "coroutines",
	"UNSAFE_CAST_WHEN_NULLABLE":    "potentialbugs",
	"SMOKE_CLASS":                  "fir",
}

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
}

// ToScannerFinding converts a FirFinding to a scanner.Finding.
// Known FIR diagnostics are normalized back to Krit catalog rule IDs so
// --fir findings deduplicate with the Go implementations while Track B runs
// both versions side by side.
func ToScannerFinding(f FirFinding) scanner.Finding {
	sev := f.Severity
	if sev == "" {
		sev = "warning"
	}
	rule := f.Rule
	if mapped := firDiagnosticRuleNames[f.Rule]; mapped != "" {
		rule = mapped
	}
	ruleSet := firDiagnosticRuleSets[f.Rule]
	if ruleSet == "" {
		ruleSet = "fir"
	}
	return scanner.Finding{
		File:       f.Path,
		Line:       f.Line,
		Col:        f.Col,
		StartByte:  f.StartByte,
		EndByte:    f.EndByte,
		RuleSet:    ruleSet,
		Rule:       rule,
		Severity:   sev,
		Message:    f.Message,
		Confidence: f.Confidence,
	}
}
