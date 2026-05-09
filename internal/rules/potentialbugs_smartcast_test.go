package rules_test

import (
	"testing"
)

func TestSmartCastInvalidated_ReassignedAndUsed_Positive(t *testing.T) {
	findings := runRuleByName(t, "SmartCastInvalidated", `
package test
fun main(): Int {
    var x: String? = "hi"
    if (x != null) {
        x = null
        return x.length
    }
    return 0
}
`)
	if len(findings) == 0 {
		t.Fatal("expected SmartCastInvalidated finding for reassigned var")
	}
}

func TestSmartCastInvalidated_NoReassignment_Negative(t *testing.T) {
	findings := runRuleByName(t, "SmartCastInvalidated", `
package test
fun main(): Int {
    var x: String? = "hi"
    if (x != null) {
        return x.length
    }
    return 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings without reassignment, got %d", len(findings))
	}
}

func TestSmartCastInvalidated_ValDeclaration_Negative(t *testing.T) {
	findings := runRuleByName(t, "SmartCastInvalidated", `
package test
fun main(s: String?): Int {
    if (s != null) {
        return s.length
    }
    return 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings on parameter (val), got %d", len(findings))
	}
}

func TestSmartCastInvalidated_SafeCallAfterReassignment_Negative(t *testing.T) {
	findings := runRuleByName(t, "SmartCastInvalidated", `
package test
fun main(): Int {
    var x: String? = "hi"
    if (x != null) {
        x = null
        return x?.length ?: 0
    }
    return 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when safe-call used, got %d", len(findings))
	}
}

func TestSmartCastInvalidated_UseBeforeReassignment_Negative(t *testing.T) {
	findings := runRuleByName(t, "SmartCastInvalidated", `
package test
fun main(): Int {
    var x: String? = "hi"
    if (x != null) {
        val len = x.length
        x = null
        return len
    }
    return 0
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when use precedes reassignment, got %d", len(findings))
	}
}
