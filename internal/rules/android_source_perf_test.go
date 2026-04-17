package rules_test

import (
	"testing"
)

// =====================================================================
// SparseArrayRule ("UseSparseArrays")
// =====================================================================

func TestSparseArrayRule(t *testing.T) {
	t.Run("HashMap_Int_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
val map = HashMap<Int, String>()
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("HashMap_Long_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
val map = HashMap<Long, String>()
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if got := findings[0].Message; got == "" {
			t.Fatal("expected non-empty message")
		}
	})

	t.Run("HashMap_String_key_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
val map = HashMap<String, Int>()
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no_HashMap_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseSparseArrays", `
package test
val list = listOf(1, 2, 3)
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// UseValueOfRule ("UseValueOf")
// =====================================================================

func TestUseValueOfRule(t *testing.T) {
	t.Run("Integer_constructor_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
fun foo() {
    val x = Integer(42)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Boolean_constructor_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
fun foo() {
    val x = Boolean(true)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("import_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
import java.lang.Integer
fun foo() {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("type_annotation_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
fun foo() {
    val x: Integer = getValue()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// ToastRule ("ShowToast")
// =====================================================================

func TestShowToastRule(t *testing.T) {
	t.Run("makeText_without_show_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
fun foo() {
    Toast.makeText(ctx, "hi", Toast.LENGTH_SHORT)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("makeText_with_show_same_line_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
fun foo() {
    Toast.makeText(ctx, "hi", Toast.LENGTH_SHORT).show()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("makeText_with_show_3_lines_later_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
fun foo() {
    val toast = Toast.makeText(ctx, "hi", Toast.LENGTH_SHORT)
    val x = 1
    val y = 2
    toast.show()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// ServiceCastRule ("ServiceCast")
// =====================================================================

func TestServiceCastRule(t *testing.T) {
	t.Run("wrong_cast_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
fun foo() {
    val mgr = getSystemService(ALARM_SERVICE) as PowerManager
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("correct_cast_alarm_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
fun foo() {
    val mgr = getSystemService(ALARM_SERVICE) as AlarmManager
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("correct_cast_power_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
fun foo() {
    val mgr = getSystemService(POWER_SERVICE) as PowerManager
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
