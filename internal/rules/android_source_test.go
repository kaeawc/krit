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

func TestFragmentConstructor(t *testing.T) {
	t.Run("parameterized primary constructor without default triggers", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment(val id: Int) : Fragment() {
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("no explicit constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment : Fragment() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("all default parameters does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment(val id: Int = 0, val name: String = "") : Fragment() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("secondary no-arg constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment : Fragment {
    constructor() : super()
    constructor(id: Int) : super()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-fragment class does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
class MyHelper(val id: Int) {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("parameterized primary plus secondary no-arg constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment(val id: Int) : Fragment() {
    constructor() : this(0)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("inject-annotated primary constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
import javax.inject.Inject
class MyFragment @Inject constructor(private val repo: Repo) : Fragment() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("assisted-inject primary constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment @AssistedInject constructor(@Assisted val id: Int, private val repo: Repo) : Fragment() {
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("inject-annotated secondary constructor does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "FragmentConstructor", `
package test
import androidx.fragment.app.Fragment
class MyFragment : Fragment {
    @Inject constructor(repo: Repo) : super()
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestServiceCast(t *testing.T) {
	t.Run("flags wrong service cast", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as PowerManager
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("message and severity follow Lint", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as PowerManager
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		want := "Suspicious cast to `PowerManager` for a `ALARM_SERVICE`: expected `AlarmManager`"
		if findings[0].Message != want {
			t.Fatalf("message = %q, want %q", findings[0].Message, want)
		}
		if findings[0].Severity != "error" {
			t.Fatalf("severity = %q, want error", findings[0].Severity)
		}
	})

	t.Run("services beyond the classic set are checked", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(Context.USAGE_STATS_SERVICE) as UserManager
        val ok = getSystemService(Context.USAGE_STATS_SERVICE) as UsageStatsManager
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("correct service cast does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as AlarmManager
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no getSystemService does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ServiceCast", `
package test
class Foo {
    fun setup() {
        val mgr = getManager() as PowerManager
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// Oracle resolves `getSystemService(...)` to a non-Context callable → suppressed.
func TestServiceCast_OracleSuppressesNonContext(t *testing.T) {
	findings := runServiceCastWithCallTarget(t, `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as PowerManager
    }
}
`, "getSystemService", "com.acme.local.Registry.getSystemService")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-Context getSystemService, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms android.content.Context.getSystemService → fires.
func TestServiceCast_OracleConfirmsContext(t *testing.T) {
	findings := runServiceCastWithCallTarget(t, `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as PowerManager
    }
}
`, "getSystemService", "android.content.Context.getSystemService")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed Context.getSystemService to fire, got %d: %v", len(findings), findings)
	}
}

// Pins NeedsOracleCallTargets on the rule.
func TestServiceCast_DeclaresOracleCallTargets(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "ServiceCast" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("ServiceCast rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
}

func runServiceCastWithCallTarget(t *testing.T, code string, callText string, target string) []scanner.Finding {
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
		if r.ID == "ServiceCast" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule not found in registry")
	return nil
}

func TestServiceCast_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runServiceCastWithCallTarget(t, `
package test
class Foo {
    fun setup() {
        val mgr = getSystemService(ALARM_SERVICE) as PowerManager
    }
}
`, "getSystemService", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress ServiceCast, got %d: %v", len(findings), findings)
	}
}

func TestShowToast(t *testing.T) {
	t.Run("flags Toast.makeText without show", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Toast.makeText with chained show does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).show()
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no Toast usage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "ShowToast", `
package test
class Foo {
    fun doStuff() {
        println("hello")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// Oracle resolves `Toast.makeText` to a non-Android callable → suppressed.
func TestShowToast_OracleSuppressesNonAndroidToast(t *testing.T) {
	findings := runShowToastWithCallTarget(t, `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }
}
`, "Toast.makeText", "com.acme.local.Toast.makeText")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-Android Toast.makeText, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms android.widget.Toast.makeText → fires.
func TestShowToast_OracleConfirmsAndroidToast(t *testing.T) {
	findings := runShowToastWithCallTarget(t, `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }
}
`, "Toast.makeText", "android.widget.Toast.makeText")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed Toast.makeText to fire, got %d: %v", len(findings), findings)
	}
}

func TestShowToast_DeclaresOracleCallTargets(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "ShowToast" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("ShowToast rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
}

func runShowToastWithCallTarget(t *testing.T, code string, callText string, target string) []scanner.Finding {
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
		if r.ID == "ShowToast" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule not found in registry")
	return nil
}

func TestShowToast_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runShowToastWithCallTarget(t, `
package test
class Foo {
    fun notify() {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }
}
`, "Toast.makeText", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress ShowToast, got %d: %v", len(findings), findings)
	}
}

func TestUseValueOf(t *testing.T) {
	t.Run("flags Integer constructor call", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val num = Integer(42)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("flags Boolean constructor call", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val flag = Boolean(true)
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("valueOf usage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val num = Integer.valueOf(42)
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("flags new Integer in Java source", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Integer num = new Integer(42);
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("Java valueOf does not trigger", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Integer num = Integer.valueOf(42);
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("message names the replacement call like Lint", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Long n = new Long(5L);
}`)
		if len(findings) != 1 || findings[0].Message != "Use `Long.valueOf(5L)` instead" {
			t.Fatalf("unexpected findings: %+v", findings)
		}
	})

	t.Run("flags java.lang qualified constructors in Java and Kotlin", func(t *testing.T) {
		if f := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Number n = new java.lang.Integer(5);
}`); len(f) != 1 {
			t.Fatalf("expected 1 Java finding, got %d", len(f))
		}
		if f := runRuleByName(t, "UseValueOf", `
package test
class Foo {
    val n = java.lang.Integer(5)
}`); len(f) != 1 {
			t.Fatalf("expected 1 Kotlin finding, got %d", len(f))
		}
	})

	t.Run("Short constructors are not reported like Lint", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Short n = new Short((short) 1);
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("Java anonymous subclass does not trigger", func(t *testing.T) {
		findings := runRuleByNameOnJava(t, "UseValueOf", `
package test;
class Foo {
    Object o = new Integer(42) { };
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLogTag(t *testing.T) {
	t.Run("flags tag over 23 characters", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    fun log() {
        Log.d("ThisIsAVeryLongTagNameThatExceedsTwentyThreeChars", "message")
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("tag within limit does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    fun log() {
        Log.d("MyTag", "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("tag exactly 23 chars does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LongLogTag", `
package test
class Foo {
    fun log() {
        Log.d("12345678901234567890123", "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLogTagMismatch(t *testing.T) {
	t.Run("flags TAG value not matching class name", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }
    fun doStuff() {
        Log.d(TAG, "message")
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("TAG matching class name does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    companion object {
        const val TAG = "MyActivity"
    }
    fun doStuff() {
        Log.d(TAG, "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no TAG constant does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "LogTagMismatch", `
package test
class MyActivity {
    fun doStuff() {
        Log.d("MyActivity", "message")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestNonInternationalizedSms(t *testing.T) {
	t.Run("flags SmsManager.sendTextMessage with domestic literal", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send() {
        SmsManager.sendTextMessage("5551234567", null, "Hello", null, null)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("flags smsManager.sendMultipartTextMessage with domestic literal", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send(smsManager: SmsManager) {
        smsManager.sendMultipartTextMessage("5551234567", null, parts, null, null)
    }
}`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("does not trigger for E.164 literal", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send() {
        SmsManager.sendTextMessage("+15551234567", null, "Hello", null, null)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("does not trigger for dynamic destination", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun send(smsManager: SmsManager, dest: String) {
        smsManager.sendTextMessage(dest, null, "Hello", null, null)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no SMS usage does not trigger", func(t *testing.T) {
		findings := runRuleByName(t, "NonInternationalizedSms", `
package test
class Foo {
    fun doStuff() {
        println("hello")
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLayoutInflation(t *testing.T) {
	t.Run("flags inflate with null parent", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `
package test
class Foo {
    fun onCreateView(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.main, null)
    }
}`, layoutInflationTestIndex())
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null parent")
		}
	})

	t.Run("ignores inflate with proper parent", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `
package test
class Foo {
    fun onCreateView(inflater: LayoutInflater, container: ViewGroup?): View {
        return inflater.inflate(R.layout.main, container, false)
    }
}`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for inflate with parent, got %d", len(findings))
		}
	})
}

func TestGetSignatures(t *testing.T) {
	t.Run("flags GET_SIGNATURES package info flag usage", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `
package test
class Foo {
    fun check(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }
}`)
		if len(findings) == 0 {
			t.Fatal("expected finding for GET_SIGNATURES")
		}
	})

	t.Run("ignores GET_SIGNING_CERTIFICATES", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `
package test
class Foo {
    fun check(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    }
}`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for GET_SIGNING_CERTIFICATES, got %d", len(findings))
		}
	})
}

func TestFragmentConstructor_TestSourceAndLaunchedInstanceIgnored(t *testing.T) {
	code := `
package test
import androidx.fragment.app.Fragment
class ArgFragment(val x: Int) : Fragment()
`
	plain := runRuleByName(t, "FragmentConstructor", code)
	if len(plain) != 1 {
		t.Fatalf("expected 1 finding in main source, got %d", len(plain))
	}
	inTest := runRuleByNameOnPath(t, "FragmentConstructor", "app/src/test/kotlin/ArgFragment.kt", code)
	if len(inTest) != 0 {
		t.Errorf("expected no findings in a test source set, got %d", len(inTest))
	}
	launched := runRuleByName(t, "FragmentConstructor", code+`
fun check() { launchFragment(ArgFragment(1)) }
`)
	if len(launched) != 0 {
		t.Errorf("expected no findings for a launchFragment(instance) fragment, got %d", len(launched))
	}
}

func TestFragmentConstructor_SecondaryOnlyConstructors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"secondary constructor only with params", "class MyFragment : Fragment { constructor(x: Int) : super() }", 1},
		{"secondary constructor only with no-arg", "class MyFragment : Fragment { constructor() : super() }", 0},
		{"implicit primary plus param secondary", "class MyFragment : Fragment() { constructor(x: Int) : this() }", 0},
		{"secondary with all defaults gets no JVM overload", "class MyFragment : Fragment { constructor(x: Int = 1) : super() }", 1},
		{"secondary with all defaults and JvmOverloads", "class MyFragment : Fragment { @JvmOverloads constructor(x: Int = 1) : super() }", 0},
		{"no-arg beside private param constructor", "class MyFragment private constructor(x: Int) : Fragment() { constructor() : this(0) }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "FragmentConstructor", "package test\nimport androidx.fragment.app.Fragment\n"+tc.body+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}

func TestGetSignatures_FlagOwnerAndJava(t *testing.T) {
	findings := runRuleByName(t, "GetSignatures", `
package test
import android.content.pm.PackageManager
fun f(pm: PackageManager, other: Other) {
    pm.getPackageInfo("a", PackageManager.GET_SIGNATURES)
    other.getPackageInfo("a", Other.GET_SIGNATURES)
    pm.getPackageInfo("a", GET_SIGNATURES)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the PackageManager flag to be reported, got %v", findings)
	}
	java := runJavaRuleByName(t, "GetSignatures", `
package test;
import android.content.pm.PackageManager;
class A {
    void f(PackageManager pm) throws Exception {
        pm.getPackageInfo("a", PackageManager.GET_SIGNATURES);
        pm.getPackageInfo("a", PackageManager.GET_SIGNING_CERTIFICATES);
        pm.getPackageInfo("a", Other.GET_SIGNATURES);
    }
}
`)
	if len(java) != 1 {
		t.Fatalf("expected 1 Java finding, got %v", java)
	}
}
