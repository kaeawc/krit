package firchecks

import (
	"fmt"
	"strings"
)

// incrementalFIRDiagnostic reports planner status, environment opt-out,
// compiler errors, and snapshot persistence in that precedence order. A plan
// is reported unless compiler errors gated its compile; without a plan, an
// explicit environment opt-out precedes response-backed error telemetry, and
// --no-cache is appended last.
func incrementalFIRDiagnostic(result *Result, noCache bool, environment string) string {
	if result == nil {
		return ""
	}
	message := ""
	plan := result.Incremental
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "0", "false", "off":
		if plan == nil {
			message = "incremental FIR off: KRIT_FIR_INCREMENTAL=" + strings.TrimSpace(environment)
		}
	}
	if message == "" && (plan == nil || plan.Path == "full") {
		switch {
		case plan != nil && plan.Reason != "tiered_rules" && result.incrementalErrorFiles > 0:
			message = "incremental FIR off: " + compilerErrorFilesDiagnostic(result.incrementalErrorFiles) + ", no snapshot"
		case plan != nil:
			message = "incremental FIR: " + plan.String()
		case result.responseApplied && result.incrementalErrorFiles > 0:
			message = "incremental FIR: " + compilerErrorFilesDiagnostic(result.incrementalErrorFiles) + " (planner telemetry unavailable)"
		}
	}
	if noCache {
		if message != "" {
			message += "; "
		}
		message += "incremental FIR snapshot persistence off: --no-cache"
	}
	return message
}

func compilerErrorFilesDiagnostic(count int) string {
	noun := "files"
	if count == 1 {
		noun = "file"
	}
	return fmt.Sprintf("%d requested %s gated by compiler errors", count, noun)
}
