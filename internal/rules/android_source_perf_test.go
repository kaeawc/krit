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

	t.Run("makeText_with_unrelated_show_still_triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
fun foo() {
    val toast = Toast.makeText(ctx, "hi", Toast.LENGTH_SHORT)
    dialog.show()
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("makeText_with_apply_lambda_show_no_trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
fun foo() {
    Toast.makeText(ctx, "hi", Toast.LENGTH_SHORT).apply {
        show()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestShowToastRule_Escapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"expression_body_returned", "fun make(): Toast = Toast.makeText(ctx, \"hi\", 0)", 0},
		{"return_statement", "fun make(): Toast {\n    return Toast.makeText(ctx, \"hi\", 0)\n}", 0},
		{"passed_as_argument", "fun foo() {\n    register(Toast.makeText(ctx, \"hi\", 0))\n}", 0},
		{"assigned_to_field", "fun foo() {\n    held = Toast.makeText(ctx, \"hi\", 0)\n}", 0},
		{"also_it_show", "fun foo() {\n    Toast.makeText(ctx, \"hi\", 0).also { it.show() }\n}", 0},
		{"let_it_show", "fun foo() {\n    Toast.makeText(ctx, \"hi\", 0).let { it.show() }\n}", 0},
		{"run_show", "fun foo() {\n    Toast.makeText(ctx, \"hi\", 0).run { show() }\n}", 0},
		{"also_unrelated_show_triggers", "fun foo() {\n    Toast.makeText(ctx, \"hi\", 0).also { dialog.show() }\n}", 1},
		{"unused_local_triggers", "fun foo() {\n    val t = Toast.makeText(ctx, \"hi\", 0)\n}", 1},
		{"setter_chain_unshown_triggers", "fun foo() {\n    Toast.makeText(ctx, \"hi\", 0).setGravity(0, 0, 0)\n}", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "ShowToast", "package test\n"+tc.body+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
			}
		})
	}
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

func TestSparseArrayRule_EscapingMapIgnored(t *testing.T) {
	findings := runRuleByName(t, "UseSparseArrays", `
package test
class Cache {
    private val local = HashMap<Int, String>()
    private val escaped = HashMap<Int, String>()
    private val cached = HashMap<Int, String>()
    private val typed: MutableMap<Int, String> = HashMap<Int, String>()

    fun use() {
        local[1] = "a"
        consume(escaped)
        cached.getOrPut(2) { "x" }
        consume(HashMap<Int, String>())
    }

    fun consume(m: Map<Int, String>) {}
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the local-only map to be flagged, got %d: %v", len(findings), findings)
	}
}
