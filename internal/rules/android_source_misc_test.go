package rules_test

import (
	"testing"
)

// =====================================================================
// LogTagLengthRule ("LongLogTag")
// =====================================================================

func TestLongLogTagRule(t *testing.T) {
	t.Run("tag exceeding 23 chars triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
fun foo() {
    Log.d("ThisIsAVeryLongTagNameThatExceeds", "msg")
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("short tag does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
fun foo() {
    Log.d("ShortTag", "msg")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("TAG variable reference does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
fun foo() {
    Log.d(TAG, "msg")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// LogTagMismatchRule ("LogTagMismatch")
// =====================================================================

func TestLogTagMismatchRule(t *testing.T) {
	t.Run("mismatched TAG triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }
    fun foo() {
        Log.d(TAG, "message")
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for mismatched TAG")
		}
	})

	t.Run("matching TAG does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "MyActivity"
    }
    fun foo() {
        Log.d(TAG, "message")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no TAG constant does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    fun foo() {
        Log.d("MyActivity", "message")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// NonInternationalizedSmsRule ("NonInternationalizedSms")
// =====================================================================

func TestNonInternationalizedSmsRule(t *testing.T) {
	t.Run("smsManager.sendTextMessage triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo() {
    smsManager.sendTextMessage(dest, null, msg, null, null)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("SmsManager.getDefault().sendTextMessage triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo() {
    SmsManager.getDefault().sendTextMessage(dest, null, msg, null, null)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("messageService.sendTextMessage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo() {
    messageService.sendTextMessage(dest, null, msg, null, null)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// ViewConstructorRule ("ViewConstructor")
// =====================================================================

func TestViewConstructorRule(t *testing.T) {
	t.Run("missing AttributeSet constructor triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ViewConstructor", `
package test
class MyView(context: Context) : View(context) {
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("has Context and AttributeSet does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ViewConstructor", `
package test
class MyView(context: Context, attrs: AttributeSet) : View(context, attrs) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("JvmOverloads does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ViewConstructor", `
package test
class MyView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0
) : View(context, attrs, defStyleAttr) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("abstract class does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ViewConstructor", `
package test
abstract class BaseView(context: Context) : View(context) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-View class does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ViewConstructor", `
package test
class NotAView(val x: Int) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// =====================================================================
// ViewTagRule ("ViewTag")
// =====================================================================

func TestViewTagRule(t *testing.T) {
	t.Run("setTag with activity triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ViewTag", `
package test
fun foo() {
    view.setTag(activity)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("setTag with drawable triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ViewTag", `
package test
fun foo() {
    view.setTag(drawable)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("setTag with plain int does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ViewTag", `
package test
fun foo() {
    holder.setTag(42)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("setTag with activityManager does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ViewTag", `
package test
fun foo() {
    view.setTag(activityManager)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
