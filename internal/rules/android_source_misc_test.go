package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
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

	t.Run("self class::class.java.simpleName does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        private val TAG = MyActivity::class.java.simpleName
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

	t.Run("other class::class.java.simpleName triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class PaymentFragment {
    companion object {
        private val TAG = CheckoutActivity::class.java.simpleName
    }
    fun foo() {
        Log.d(TAG, "message")
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for mismatched ::class.java.simpleName")
		}
	})
}

// =====================================================================
// NonInternationalizedSmsRule ("NonInternationalizedSms")
// =====================================================================

func TestNonInternationalizedSmsRule(t *testing.T) {
	t.Run("smsManager.sendTextMessage with domestic literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo() {
    smsManager.sendTextMessage("5551234567", null, msg, null, null)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("SmsManager.getDefault().sendTextMessage with domestic literal triggers", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo() {
    SmsManager.getDefault().sendTextMessage("5551234567", null, msg, null, null)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("dynamic destination does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo(dest: String) {
    smsManager.sendTextMessage(dest, null, msg, null, null)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("messageService.sendTextMessage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
fun foo() {
    messageService.sendTextMessage("5551234567", null, msg, null, null)
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

func runViewTagRule(t *testing.T, code string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	return runViewTagRuleWithResolver(t, file, resolver)
}

func runViewTagRuleWithResolver(t *testing.T, file *scanner.File, resolver typeinfer.TypeResolver) []scanner.Finding {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.ID == "ViewTag" {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r}, resolver)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("ViewTag rule not found")
	return nil
}

func TestViewTagRule(t *testing.T) {
	t.Run("one-arg setTag with Activity triggers", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.app.Activity
import android.view.View
fun foo(view: View, activity: Activity) {
    view.setTag(activity)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("multiline setTag with Drawable triggers", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.graphics.drawable.Drawable
import android.view.View
fun foo(view: View, drawable: Drawable) {
    view.setTag(
        drawable
    )
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("call result resolving to Context triggers", func(t *testing.T) {
		file := parseInline(t, `
package test
fun foo() {
    view.setTag(fragment.requireContext())
}
`)
		resolver := typeinfer.NewFakeResolver()
		resolver.NodeTypes["view"] = &typeinfer.ResolvedType{Name: "View", FQN: "android.view.View", Kind: typeinfer.TypeClass}
		resolver.NodeTypes["fragment.requireContext()"] = &typeinfer.ResolvedType{Name: "Context", FQN: "android.content.Context", Kind: typeinfer.TypeClass}
		findings := runViewTagRuleWithResolver(t, file, resolver)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("keyed setTag overload does not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.app.Activity
import android.view.View
object R { object id { const val owner = 1 } }
fun foo(view: View, activity: Activity) {
    view.setTag(R.id.owner, activity)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unrelated setTag method does not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.app.Activity
class TagStore { fun setTag(value: Activity) {} }
fun foo(store: TagStore, activity: Activity) {
    store.setTag(activity)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("string literal and string variable do not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.view.View
fun foo(view: View, contextName: String) {
    view.setTag("activity")
    view.setTag(contextName)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("comments and strings do not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
fun foo() {
    // view.setTag(activity)
    val sample = "view.setTag(activity)"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("aliases resolve framework types", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.app.Activity as AndroidActivity
import android.view.View as AndroidView
fun foo(view: AndroidView, activity: AndroidActivity) {
    view.setTag(activity)
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("same-file View subclass triggers with lower confidence", func(t *testing.T) {
		file := parseInline(t, `
package test
fun foo() {
    view.setTag(activity)
}
`)
		resolver := typeinfer.NewFakeResolver()
		resolver.NodeTypes["view"] = &typeinfer.ResolvedType{Name: "MyView", FQN: "test.MyView", Kind: typeinfer.TypeClass}
		resolver.NodeTypes["activity"] = &typeinfer.ResolvedType{Name: "Activity", FQN: "android.app.Activity", Kind: typeinfer.TypeClass}
		resolver.Classes["test.MyView"] = &typeinfer.ClassInfo{
			Name:       "MyView",
			FQN:        "test.MyView",
			Supertypes: []string{"android.view.View"},
			File:       file.Path,
		}
		findings := runViewTagRuleWithResolver(t, file, resolver)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Confidence != 0.80 {
			t.Fatalf("expected lower-confidence same-file finding, got %.2f", findings[0].Confidence)
		}
	})

	t.Run("local shadowed names do not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
class View { fun setTag(value: Any) {} }
class Activity
fun foo(view: View, activity: Activity) {
    view.setTag(activity)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("external unresolved receiver subtype does not trigger", func(t *testing.T) {
		file := parseInline(t, `
package test
fun foo() {
    view.setTag(activity)
}
`)
		resolver := typeinfer.NewFakeResolver()
		resolver.NodeTypes["view"] = &typeinfer.ResolvedType{Name: "ExternalView", FQN: "com.example.ExternalView", Kind: typeinfer.TypeClass}
		resolver.NodeTypes["activity"] = &typeinfer.ResolvedType{Name: "Activity", FQN: "android.app.Activity", Kind: typeinfer.TypeClass}
		resolver.Classes["com.example.ExternalView"] = &typeinfer.ClassInfo{
			Name:       "ExternalView",
			FQN:        "com.example.ExternalView",
			Supertypes: []string{"android.view.View"},
			File:       "/other/ExternalView.kt",
		}
		findings := runViewTagRuleWithResolver(t, file, resolver)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unresolved receiver does not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.app.Activity
fun foo(activity: Activity) {
    owner.setTag(activity)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unresolved argument does not trigger", func(t *testing.T) {
		findings := runViewTagRule(t, `
package test
import android.view.View
fun foo(view: View) {
    view.setTag(activity)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
