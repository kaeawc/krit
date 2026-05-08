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

func TestUnusedResources(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "UnusedResources", `
package test

fun load() {
    val id = R.string.test_placeholder
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "UnusedResources", `
package test

fun load() {
    val id = R.string.welcome_message
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
