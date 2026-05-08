package scan

import (
	"testing"
)

func TestRunSampleRuleShortCircuitNoOpWhenEmpty(t *testing.T) {
	handled, code := runSampleRuleShortCircuit(nil, sampleRuleOpts{Rule: ""})
	if handled {
		t.Errorf("handled = true; want false when Rule is empty")
	}
	if code != 0 {
		t.Errorf("code = %d; want 0 in the no-op path", code)
	}
}

func TestRunRuleAuditShortCircuitNoOpWhenDisabled(t *testing.T) {
	handled, code := runRuleAuditShortCircuit(nil, false, RuleAuditOpts{})
	if handled {
		t.Errorf("handled = true; want false when enabled=false")
	}
	if code != 0 {
		t.Errorf("code = %d; want 0 in the no-op path", code)
	}
}
