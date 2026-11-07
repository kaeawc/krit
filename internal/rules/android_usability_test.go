package rules_test

import "testing"

func TestNewApi(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NewApi", `
package test

fun setup() {
    val channel = NotificationChannel("id", "name", importance)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "NewApi", `
package test

fun setup() {
    val view = TextView(context)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestInlinedApi(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "InlinedApi", `
package test

fun setup() {
    val flags = SYSTEM_UI_FLAG_FULLSCREEN
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "InlinedApi", `
package test

fun setup() {
    val x = 42
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestOverride(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Override", `
package test

class MyActivity : Activity() {
    fun onBackPressed() {
        finish()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("Fragment helper named onBackPressed is not an override", func(t *testing.T) {
		findings := runRuleByName(t, "Override", `
package test

class MyFragment : Fragment() {
    fun onBackPressed() {}
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("Fragment onOptionsItemSelected still triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Override", `
package test

class MyFragment : Fragment() {
    fun onOptionsItemSelected(item: MenuItem): Boolean = true
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "Override", `
package test

class MyActivity : Activity() {
    override fun onBackPressed() {
        finish()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
