package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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

	t.Run("unresolved TAG variable reference does not trigger", func(t *testing.T) {
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

	t.Run("TAG const val exceeding 23 chars triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class NetworkRepositoryImpl {
    companion object {
        private const val TAG = "NetworkRepositoryImpl_v2"
    }
    fun foo() {
        Log.d(TAG, "msg")
    }
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("TAG const val within limit does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class NetworkRepositoryImpl {
    companion object {
        private const val TAG = "NetworkRepository"
    }
    fun foo() {
        Log.d(TAG, "msg")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("interpolated TAG initializer does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    companion object {
        val TAG = "prefix_${'$'}suffixThatIsActuallyVeryLongIndeed"
    }
    fun foo() {
        Log.d(TAG, "msg")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("top-level const val tag exceeding 23 chars triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
private const val LONG_TAG_NAME = "ThisIsAVeryLongTopLevelTag"
fun foo() {
    Log.d(LONG_TAG_NAME, "msg")
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

// Oracle resolves the Log call to a non-Android callable → suppressed.
func TestLongLogTag_OracleSuppressesNonAndroidLog(t *testing.T) {
	findings := runLogTagWithCallTarget(t, "LongLogTag", `
package test
fun foo() {
    Log.d("ThisIsAVeryLongTagNameThatExceeds", "msg")
}
`, "Log.d", "com.acme.local.Log.d")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-Android Log.d, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms android.util.Log.d → fires.
func TestLongLogTag_OracleConfirmsAndroidLog(t *testing.T) {
	findings := runLogTagWithCallTarget(t, "LongLogTag", `
package test
fun foo() {
    Log.d("ThisIsAVeryLongTagNameThatExceeds", "msg")
}
`, "Log.d", "android.util.Log.d")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed Log.d to fire, got %d: %v", len(findings), findings)
	}
}

func TestLongLogTag_DeclaresOracleCallTargets(t *testing.T) {
	assertLogTagOracleCapability(t, "LongLogTag")
}

// Oracle resolves the Log.d(TAG, …) call to a non-Android callable → suppressed.
func TestLogTagMismatch_OracleSuppressesNonAndroidLog(t *testing.T) {
	findings := runLogTagWithCallTarget(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }
    fun foo() {
        Log.d(TAG, "message")
    }
}
`, "Log.d", "com.acme.local.Log.d")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-Android Log.d, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms android.util.Log.d → fires.
func TestLogTagMismatch_OracleConfirmsAndroidLog(t *testing.T) {
	findings := runLogTagWithCallTarget(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }
    fun foo() {
        Log.d(TAG, "message")
    }
}
`, "Log.d", "android.util.Log.d")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed Log.d to fire, got %d: %v", len(findings), findings)
	}
}

func TestLogTagMismatch_DeclaresOracleCallTargets(t *testing.T) {
	assertLogTagOracleCapability(t, "LogTagMismatch")
}

func assertLogTagOracleCapability(t *testing.T, ruleID string) {
	t.Helper()
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == ruleID {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatalf("%s rule not registered", ruleID)
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("%s missing NeedsOracleCallTargets: Needs=%b", ruleID, rule.Needs)
	}
}

func runLogTagWithCallTarget(t *testing.T, ruleID, code, callText, target string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if target != "" && strings.Contains(strings.TrimSpace(file.FlatNodeText(idx)), callText) {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == ruleID {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %s not found in registry", ruleID)
	return nil
}

func TestLongLogTag_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runLogTagWithCallTarget(t, "LongLogTag", `
package test
fun foo() {
    Log.d("ThisIsAVeryLongTagNameThatExceeds", "msg")
}
`, "Log.d", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress LongLogTag, got %d: %v", len(findings), findings)
	}
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
		findings := runViewConstructorRule(t, `
package test
class MyView(context: Context) : View(context) {
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("has Context and AttributeSet does not trigger", func(t *testing.T) {
		findings := runViewConstructorRule(t, `
package test
class MyView(context: Context, attrs: AttributeSet) : View(context, attrs) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("JvmOverloads does not trigger", func(t *testing.T) {
		findings := runViewConstructorRule(t, `
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
		findings := runViewConstructorRule(t, `
package test
abstract class BaseView(context: Context) : View(context) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-View class does not trigger", func(t *testing.T) {
		findings := runViewConstructorRule(t, `
package test
class NotAView(val x: Int) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("fake implementing nested Contract.View interface does not trigger", func(t *testing.T) {
		findings := runViewConstructorRule(t, `
package test
class FakeScreen(val title: String) : Contract.View {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("imported non-Android View interface does not trigger", func(t *testing.T) {
		findings := runViewConstructorRule(t, `
package test
import com.example.Contract.View
class FakeScreen(val title: String) : View {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("SuppressLint ViewConstructor does not trigger", func(t *testing.T) {
		findings := runViewConstructorRule(t, `
package test
@SuppressLint("ViewConstructor")
class MyView(context: Context) : View(context) {
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("test source set does not trigger", func(t *testing.T) {
		findings := runViewConstructorRuleOnPath(t, "src/test/kotlin/MyView.kt", `
package test
class MyView(context: Context) : View(context) {
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
	for _, r := range api.Registry {
		if r.ID == "ViewTag" {
			dispatcher := rules.NewDispatcher([]*api.Rule{r}, resolver)
			cols := dispatcher.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("ViewTag rule not found")
	return nil
}

func TestViewTagRule(t *testing.T) {
	trigger := func(t *testing.T, name, code string, want int) {
		t.Run(name, func(t *testing.T) {
			findings := runViewTagRule(t, code)
			if len(findings) != want {
				t.Fatalf("expected %d findings, got %d", want, len(findings))
			}
		})
	}

	trigger(t, "keyed setTag with a View triggers", `
package test
import android.view.View
fun foo(view: View, other: View) {
    view.setTag(1, other)
}
`, 1)
	trigger(t, "keyed setTag with a Cursor triggers", `
package test
import android.database.Cursor
import android.view.View
fun foo(view: View, cursor: Cursor) {
    view.setTag(1, cursor)
}
`, 1)
	trigger(t, "keyed setTag with a RecyclerView ViewHolder triggers", `
package test
import androidx.recyclerview.widget.RecyclerView.ViewHolder
import android.view.View
fun foo(view: View, holder: ViewHolder) {
    view.setTag(1, holder)
}
`, 1)
	trigger(t, "multiline keyed setTag triggers", `
package test
import android.view.View
fun foo(view: View, other: View) {
    view.setTag(
        1,
        other
    )
}
`, 1)
	trigger(t, "one-arg setTag never leaks and does not trigger", `
package test
import android.view.View
fun foo(view: View, other: View) {
    view.setTag(other)
}
`, 0)
	trigger(t, "keyed setTag with Activity or String does not trigger", `
package test
import android.app.Activity
import android.view.View
fun foo(view: View, activity: Activity) {
    view.setTag(1, activity)
    view.setTag(2, "label")
}
`, 0)
	trigger(t, "unrelated setTag method does not trigger", `
package test
import android.view.View
class TagStore { fun setTag(key: Int, value: View) {} }
fun foo(store: TagStore, other: View) {
    store.setTag(1, other)
}
`, 0)
	trigger(t, "local shadowed names do not trigger", `
package test
class View { fun setTag(key: Int, value: Any) {} }
fun foo(view: View, other: View) {
    view.setTag(1, other)
}
`, 0)
	trigger(t, "unresolved receiver does not trigger", `
package test
import android.view.View
fun foo(other: View) {
    owner.setTag(1, other)
}
`, 0)
	trigger(t, "comments and strings do not trigger", `
package test
fun foo() {
    // view.setTag(1, other)
    val sample = "view.setTag(1, other)"
}
`, 0)
	trigger(t, "aliases resolve framework types", `
package test
import android.view.View as AndroidView
fun foo(view: AndroidView, other: AndroidView) {
    view.setTag(1, other)
}
`, 1)
	trigger(t, "verify does not trigger", `
package test
import android.view.View
fun foo(view: View, other: View) {
    verify(view).setTag(1, other)
}
`, 0)

	t.Run("setTag in a test source file does not trigger", func(t *testing.T) {
		file := parseInline(t, `
package test
import android.view.View
fun foo(view: View, other: View) {
    view.setTag(1, other)
}
`)
		file.Path = "/repo/app/src/test/java/test/ViewTagTest.kt"
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		if findings := runViewTagRuleWithResolver(t, file, resolver); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("same-file View subclass receiver triggers with lower confidence", func(t *testing.T) {
		file := parseInline(t, `
package test
fun foo() {
    view.setTag(1, other)
}
`)
		resolver := typeinfer.NewFakeResolver()
		resolver.NodeTypes["view"] = &typeinfer.ResolvedType{Name: "MyView", FQN: "test.MyView", Kind: typeinfer.TypeClass}
		resolver.NodeTypes["other"] = &typeinfer.ResolvedType{Name: "View", FQN: "android.view.View", Kind: typeinfer.TypeClass}
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
}

func TestViewConstructorRule_ConstructedInCodeIgnored(t *testing.T) {
	findings := runViewConstructorRule(t, `
package test
class PlainView(context: Context, val extra: Int) : View(context)
fun build(context: Context) = PlainView(context, 1)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a view built only in code, got %d", len(findings))
	}
}

func TestNewApiResolvedTargetMustBeFrameworkOwner(t *testing.T) {
	const code = `
package test
import android.content.Context
fun f(ctx: Context) {
    val c = ctx.getColor(1)
}
`
	for target, want := range map[string]int{
		"android.content.Context.getColor":        1,
		"android.content.res.Resources.getColor":  0,
		"android.content.res.TypedArray.getColor": 0,
		"": 0,
	} {
		got := runLogTagWithCallTarget(t, "NewApi", code, "ctx.getColor(1)", target)
		if len(got) != want {
			t.Errorf("target %q: expected %d findings, got %d", target, want, len(got))
		}
	}
}

func TestRangeChecksSameFileAnnotationWhenOracleResolvesCall(t *testing.T) {
	const code = `
package test
import androidx.annotation.IntRange
fun m(@IntRange(from = 0, to = 10) x: Int) {}
fun f() { m(11) }
`
	for target, want := range map[string]int{
		"test.m":      1,
		"other.pkg.n": 0,
	} {
		got := runLogTagWithCallTarget(t, "Range", code, "m(11)", target)
		if len(got) != want {
			t.Errorf("target %q: expected %d findings, got %v", target, want, got)
		}
	}
}

func TestRecycleInferredLocalUsesResolvedType(t *testing.T) {
	const code = `
package test
fun f(ctx: Context) {
    val c = ctx.query()
    c.moveToFirst()
}
fun g(ctx: Context) {
    val c = ctx.query()
    c.moveToFirst()
    c.close()
}
`
	run := func(fqn string) []scanner.Finding {
		file := parseInline(t, code)
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		fake := oracle.NewFakeOracle()
		fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
		file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
			if strings.Contains(file.FlatNodeText(idx), "ctx.query()") {
				key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
				fake.Expressions[file.Path][key] = &typeinfer.ResolvedType{Name: fqn[strings.LastIndex(fqn, ".")+1:], FQN: fqn, Kind: typeinfer.TypeClass}
			}
		})
		composite := oracle.NewCompositeResolver(fake, resolver)
		for _, r := range api.Registry {
			if r.ID == "Recycle" {
				cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
				return cols.Findings()
			}
		}
		t.Fatal("Recycle not registered")
		return nil
	}
	if got := run("android.database.Cursor"); len(got) != 1 || got[0].Line != 4 {
		t.Errorf("Cursor local without close: expected one finding on line 4, got %v", got)
	}
	if got := run("com.example.Store"); len(got) != 0 {
		t.Errorf("unrelated type: expected no findings, got %v", got)
	}
}

func TestRtlAwareResolvedTargetMustBeView(t *testing.T) {
	const code = `
package test
fun f(x: Any) {
    val p = x.getPaddingLeft()
}
`
	for target, want := range map[string]int{
		"android.view.View.getPaddingLeft": 1,
		"test.Insets.getPaddingLeft":       0,
		"":                                 0,
	} {
		got := runLogTagWithCallTarget(t, "RtlAware", code, "x.getPaddingLeft()", target)
		if len(got) != want {
			t.Errorf("target %q: expected %d findings, got %d", target, want, len(got))
		}
	}
}

func TestResourceAsColorResolvedTargetMustBeFramework(t *testing.T) {
	const code = `
package test
fun f(v: Any) {
    v.setTextColor(R.color.red)
}
`
	for target, want := range map[string]int{
		"android.widget.TextView.setTextColor": 1,
		"test.Chip.setTextColor":               0,
	} {
		got := runLogTagWithCallTarget(t, "ResourceAsColor", code, "v.setTextColor", target)
		if len(got) != want {
			t.Errorf("target %q: expected %d findings, got %d", target, want, len(got))
		}
	}
}

func TestResourceTypeSkipsStyleableIndicesAndAppOwnedCalls(t *testing.T) {
	const code = `
package test
fun f(a: Any, p: Any) {
    a.getColor(R.styleable.Rt_color, 0)
    a.getString(R.drawable.icon)
    p.getString(R.drawable.icon)
}
`
	got := runLogTagWithCallTarget(t, "ResourceType", code, "a.getString(R.drawable.icon)", "android.content.Context.getString")
	if len(got) != 1 || got[0].Line != 5 {
		t.Errorf("expected only the framework getString(R.drawable) on line 5, got %v", got)
	}
	if got := runLogTagWithCallTarget(t, "ResourceType", code, "a.getColor(R.styleable.Rt_color, 0)", "android.content.res.TypedArray.getColor"); len(got) != 0 {
		t.Errorf("R.styleable indices must not be reported, got %v", got)
	}
}
