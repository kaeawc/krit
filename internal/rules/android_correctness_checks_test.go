package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestOverrideAbstractQualifiedSupertype(t *testing.T) {
	for src, want := range map[string]int{
		"class A : my.pkg.Service()":      0,
		"class A : android.app.Service()": 1,
	} {
		findings := runRuleByName(t, "OverrideAbstract", "package test\n\n"+src+"\n")
		if len(findings) != want {
			t.Errorf("%s: expected %d findings, got %d", src, want, len(findings))
		}
	}
}

func TestOverrideAbstract(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "OverrideAbstract", `
package test

class MyService : Service() {
    // missing onBind
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "OverrideAbstract", `
package test

class MyService : Service() {
    override fun onBind(intent: Intent): IBinder? = null
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestParcelCreator(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ParcelCreator", `
package test

class MyData : Parcelable {
    val name: String = ""
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ParcelCreator", `
package test

@Parcelize
class MyData : Parcelable {
    val name: String = ""
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSwitchIntDefBranchBodiesDoNotCount(t *testing.T) {
	findings := runRuleByName(t, "SwitchIntDef", `
package test
fun f(view: View) {
    when (view.visibility) {
        View.VISIBLE -> { if (a) b else c }
        View.GONE -> log("INVISIBLE VISIBLE")
    }
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "INVISIBLE") {
		t.Fatalf("expected a finding for the missing INVISIBLE constant, got %v", findings)
	}
}

func TestSwitchIntDef(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "SwitchIntDef", `
package test

fun check(visibility: Int) {
    when (visibility) {
        VISIBLE -> show()
        GONE -> hide()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "SwitchIntDef", `
package test

fun check(visibility: Int) {
    when (visibility) {
        VISIBLE -> show()
        INVISIBLE -> dim()
        GONE -> hide()
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWrongViewCastNeedsLayoutIndexAndUsesLintWording(t *testing.T) {
	for _, r := range api.Registry {
		if r.ID == "WrongViewCast" && !r.Needs.Has(api.NeedsResources) {
			t.Fatal("WrongViewCast must declare NeedsResources, or it never receives the layout index")
		}
	}
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: "TextView", ID: "@+id/tv_title", Line: 4})
	findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val implicit = findViewById<Button>(R.id.tv_title)
    val explicit = findViewById(R.id.tv_title) as Button
}
`, idx)
	if len(findings) != 2 ||
		findings[0].Message != "Unexpected implicit cast to Button: layout tag was TextView" ||
		findings[1].Message != "Unexpected cast to Button: layout tag was TextView" {
		t.Fatalf("got %+v", findings)
	}
}

func TestWrongViewCast(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			ID:   "@+id/tv_title",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val tv = findViewById<ImageView>(R.id.tv_title)
}
`, idx)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			ID:   "@+id/tv_title",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val tv = findViewById<TextView>(R.id.tv_title)
}
`, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestWrongViewCast_ASTAndResourceEvidence(t *testing.T) {
	t.Run("resource backed generic mismatch without prefix", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			ID:   "@+id/avatar",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val avatar = findViewById<TextView>(R.id.avatar)
}
`, idx)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "ImageView") {
			t.Fatalf("expected ImageView resource mismatch finding, got %#v", findings)
		}
		if findings[0].Confidence < 0.9 {
			t.Fatalf("expected high-confidence resource finding, got %.2f", findings[0].Confidence)
		}
	})

	t.Run("resource backed as-cast mismatch", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			ID:   "@+id/avatar",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val avatar = findViewById(R.id.avatar) as TextView
}
`, idx)
		if len(findings) != 1 {
			t.Fatalf("expected as-cast resource mismatch finding, got %#v", findings)
		}
	})

	t.Run("multiline generic call", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			ID:   "@+id/avatar",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val avatar = findViewById<TextView>(
        R.id.avatar
    )
}
`, idx)
		if len(findings) != 1 {
			t.Fatalf("expected multiline resource mismatch finding, got %#v", findings)
		}
	})

	t.Run("resource evidence is required over prefix fallback", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			ID:   "@+id/btn_submit",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val submit = findViewById<TextView>(R.id.btn_submit)
}
`, idx)
		if len(findings) != 0 {
			t.Fatalf("expected resource-compatible cast to stay clean despite prefix, got %#v", findings)
		}
	})

	t.Run("unresolved resource skips instead of prefix fallback", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			ID:   "@+id/title",
			Line: 4,
		})
		findings := runWrongViewCastWithResourceIndex(t, `
package test

fun setup() {
    val avatar = findViewById<TextView>(R.id.iv_avatar)
}
`, idx)
		if len(findings) != 0 {
			t.Fatalf("expected unresolved resource to skip prefix fallback with index present, got %#v", findings)
		}
	})

	t.Run("unresolved call target skips even with resource mismatch", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "ImageView",
			ID:   "@+id/avatar",
			Line: 4,
		})
		findings := runWrongViewCastWithoutCallTargets(t, `
package test

fun setup() {
    val avatar = findViewById<TextView>(R.id.avatar)
}
`, idx)
		if len(findings) != 0 {
			t.Fatalf("expected unresolved findViewById call target to skip, got %#v", findings)
		}
	})

	t.Run("unrelated local function is ignored", func(t *testing.T) {
		findings := runRuleByName(t, "WrongViewCast", `
package test

fun <T> findViewById(id: Int): T = TODO()

fun setup() {
    val view = findViewById<TextView>(R.id.btn_submit)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected local findViewById to be ignored, got %#v", findings)
		}
	})

	t.Run("comments and strings are ignored", func(t *testing.T) {
		findings := runRuleByName(t, "WrongViewCast", `
package test

fun setup() {
    // val view = findViewById<TextView>(R.id.btn_submit)
    val sample = "findViewById<TextView>(R.id.btn_submit)"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected comments and strings to be ignored, got %#v", findings)
		}
	})
}

func TestWrongViewCast_JavaForms(t *testing.T) {
	idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
		Type: "ImageView",
		ID:   "@+id/iv_avatar",
		Line: 4,
	})
	findings := runWrongViewCastOnJava(t, `
package test;

class Demo {
  void setup() {
    TextView title = findViewById(R.id.btn_submit);
    ImageView image = (ImageView) findViewById(R.id.tv_title);
    TextView required = ViewCompat.requireViewById(root, R.id.iv_avatar);
  }
}
`, idx)
	if len(findings) != 1 {
		t.Fatalf("expected only qualified ViewCompat Java finding, got %d: %#v", len(findings), findings)
	}
}

func TestDeprecated(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Deprecated", `
package test
import android.os.AsyncTask

fun doWork() {
    val task = AsyncTask()
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "Deprecated", `
package test

fun doWork() {
    val scope = CoroutineScope(Dispatchers.IO)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runWrongViewCastWithResourceIndex(t *testing.T, code string, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := runWrongViewCastResolverWithCallTargets(t, file, map[string]string{
		"findViewById":    "android.app.Activity.findViewById",
		"requireViewById": "androidx.core.view.ViewCompat.requireViewById",
	})
	return runWrongViewCastOnFile(t, file, idx, resolver)
}

func runWrongViewCastWithoutCallTargets(t *testing.T, code string, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	return runWrongViewCastOnFile(t, file, idx, nil)
}

func runWrongViewCastOnJava(t *testing.T, code string, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Test.java")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return runWrongViewCastOnFile(t, file, idx, nil)
}

func runWrongViewCastResolverWithCallTargets(t *testing.T, file *scanner.File, targets map[string]string) typeinfer.TypeResolver {
	t.Helper()
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		text := file.FlatNodeText(idx)
		for callee, target := range targets {
			if strings.Contains(text, callee) {
				key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
				fake.CallTargets[file.Path][key] = target
			}
		}
	})
	return oracle.NewCompositeResolver(fake, resolver)
}

func runWrongViewCastOnFile(t *testing.T, file *scanner.File, idx *android.ResourceIndex, resolver typeinfer.TypeResolver) []scanner.Finding {
	t.Helper()
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "WrongViewCast" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("WrongViewCast not found in registry")
	}
	wants := make(map[string]bool, len(rule.NodeTypes))
	for _, typ := range rule.NodeTypes {
		wants[typ] = true
	}
	collector := scanner.NewFindingCollector(0)
	for i := range file.FlatTree.Types {
		flatIdx := uint32(i)
		if !wants[file.FlatType(flatIdx)] {
			continue
		}
		node := file.FlatTree.Node(flatIdx)
		rule.Check(&api.Context{
			File:              file,
			Idx:               flatIdx,
			Node:              &node,
			Rule:              rule,
			ResourceIndex:     idx,
			DefaultConfidence: rule.Confidence,
			Collector:         collector,
			Resolver:          resolver,
		})
	}
	return collector.Columns().Findings()
}

func TestRange(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "Range", `
package test

fun bounded(@IntRange(from = 0, to = 10) value: Int) {}

fun apply() {
    bounded(11)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "Range", `
package test

fun bounded(@IntRange(from = 0, to = 10) value: Int) {}

fun apply() {
    bounded(10)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestResourceType(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceType", `
package test

fun load() {
    getString(R.drawable.icon)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceType", `
package test

fun load() {
    getString(R.string.hello)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestResourceAsColor(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceAsColor", `
package test

fun style() {
    view.setBackgroundColor(R.color.primary)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ResourceAsColor", `
package test

fun style() {
    view.setBackgroundColor(ContextCompat.getColor(context, R.color.primary))
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestSupportAnnotationUsage(t *testing.T) {
	cases := []struct {
		name string
		code string
		want int
	}{
		{"colorint on string parameter", "class A { fun f(@ColorInt c: String) {} }", 1},
		{"stringres on nullable boolean property", "class A { @StringRes val id: Boolean? = null }", 1},
		{"drawableres on float constructor property", "class A(@DrawableRes val id: Float)", 1},
		{"intrange on double return", "class A { @IntRange(from = 0) fun f(): Double = 0.0 }", 1},
		{"floatrange on int parameter", "class A { fun f(@FloatRange(from = 0.0) a: Int) {} }", 1},
		{"intrange on int is fine", "class A { fun f(@IntRange(from = 0) a: Int, @ColorInt b: Int?, @StringRes c: Long) {} }", 0},
		{"floatrange on double is fine", "class A { fun f(@FloatRange(from = 0.0) a: Double, @FloatRange(to = 1.0) b: Float?) {} }", 0},
		{"collections of ids are fine", "class A { fun f(@StringRes ids: List<Int>, @ColorInt colors: IntArray) {} }", 0},
		{"unannotated mismatch is ignored", "class A { fun f(a: String) {} }", 0},
		{"mainthread alone is fine", "class A { @MainThread fun f(a: String) {} }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "SupportAnnotationUsage", "package test\n"+tc.code+"\n")
			if len(findings) != tc.want {
				t.Fatalf("expected %d findings, got %d: %v", tc.want, len(findings), findings)
			}
		})
	}
}

func TestAccidentalOctal(t *testing.T) {
	r := findGradleRule(t, "AccidentalOctal")
	run := func(content string) []scanner.Finding {
		cfg, _ := android.ParseBuildGradleContent(content)
		return runGradleRule(r, "build.gradle", content, cfg)
	}
	t.Run("leading zero integer triggers", func(t *testing.T) {
		findings := run("android {\n    defaultConfig {\n        versionCode 0123\n        minSdkVersion 08\n    }\n}\n")
		if len(findings) != 2 || findings[0].Line != 3 || findings[1].Line != 4 {
			t.Fatalf("expected findings on lines 3 and 4, got %v", findings)
		}
		if findings[0].Message != "The leading 0 turns this number into octal which is probably not what was intended (interpreted as 83)" {
			t.Fatalf("unexpected message %q", findings[0].Message)
		}
		if findings[1].Message != "The leading 0 turns this number into octal which is probably not what was intended (and it is not a valid octal number)" {
			t.Fatalf("unexpected message %q", findings[1].Message)
		}
	})
	t.Run("decimals strings versions and comments are clean", func(t *testing.T) {
		findings := run("versionName \"1.05\"\nversionCode 10\nvalue 0\nratio 0.5\nx 1.05\n// versionCode 0123\nname 'a 012'\nnum = 0_1\n")
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})
	t.Run("kotlin sources are ignored", func(t *testing.T) {
		if findings := runRuleByName(t, "AccidentalOctal", "package test\n\nval perms = 0755\n"); len(findings) != 0 {
			t.Fatalf("expected no findings, got %v", findings)
		}
	})
}

func TestAppCompatMethod(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

import androidx.appcompat.app.AppCompatActivity

class Main : AppCompatActivity() {
    fun setup() {
        getActionBar()
        this.startActionMode(null)
    }
}
`)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})
	t.Run("plain Activity and foreign receivers are clean", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

import android.app.Activity
import androidx.appcompat.app.AppCompatActivity

class Plain : Activity() {
    fun setup() { getActionBar() }
}
class Main : AppCompatActivity() {
    fun setup(other: Activity) { other.getActionBar() }
}
fun free() { getActionBar() }
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

fun setup() {
    getSupportActionBar()
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("comments and strings with method names are clean", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

fun setup() {
    // getActionBar()
    val code = "setProgressBarVisibility(true)"
    val other = "setProgressBarIndeterminateVisibility(false)"
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for text lookalikes, got %d", len(findings))
		}
	})
}

func TestInnerclassSeparator(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "InnerclassSeparator", `
package test

fun load() {
    Class.forName("com/example/Outer/Inner")
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("dotted inner class triggers", func(t *testing.T) {
		findings := runRuleByName(t, "InnerclassSeparator", `
package test

fun load() {
    Class.forName("java.util.Map.Entry")
}
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
	t.Run("plain class name is clean", func(t *testing.T) {
		findings := runRuleByName(t, "InnerclassSeparator", `
package test

fun load() {
    Class.forName("com.example.Outer")
    Class.forName("com.Example.outer")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "InnerclassSeparator", `
package test

fun load() {
    Class.forName("com.example.Outer$Inner")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestObjectAnimatorBinding(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "ObjectAnimatorBinding", `
package test
import android.animation.ObjectAnimator
import android.view.View

fun animate(view: View) {
    ObjectAnimator.ofFloat(view, "fooBar", 0f, 1f)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "ObjectAnimatorBinding", `
package test
import android.animation.ObjectAnimator
import android.view.View

fun animate(view: View) {
    ObjectAnimator.ofFloat(view, "alpha", 0f, 1f)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestObjectAnimatorBindingCommonViewSetters(t *testing.T) {
	for _, prop := range []string{"backgroundColor", "background", "foreground", "translationX", "enabled"} {
		findings := runRuleByNameWithResolver(t, "ObjectAnimatorBinding", `
package test
import android.animation.ObjectAnimator
import android.view.View

fun animate(view: View) {
    ObjectAnimator.ofInt(view, "`+prop+`", 0, 1)
}
`)
		if len(findings) != 0 {
			t.Errorf("%s is a View setter, expected 0 findings, got %d", prop, len(findings))
		}
	}
}

func TestPropertyEscape(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "PropertyEscape", `
package test

val s = "hello\q world"
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "PropertyEscape", `
package test

val s = "hello\n world"
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestShortAlarm(t *testing.T) {
	run := func(args string) []scanner.Finding {
		return runRuleByName(t, "ShortAlarm", `
package test
import android.app.AlarmManager
fun schedule(alarmManager: AlarmManager, time: Long) {
    alarmManager.setRepeating(`+args+`)
}
`)
	}
	lintMessage := func(min string) string {
		return "Value will be forced up to " + min + " as of Android 5.1; don't rely on this to be exact"
	}
	t.Run("short interval reports the interval argument", func(t *testing.T) {
		findings := run("AlarmManager.RTC, 7000L, 5000, pendingIntent")
		if len(findings) != 1 || findings[0].Message != lintMessage("60000") || findings[0].Col != 56 {
			t.Fatalf("expected one interval finding at col 56, got %+v", findings)
		}
	})
	t.Run("short trigger reports the trigger argument", func(t *testing.T) {
		findings := run("AlarmManager.RTC, 1000L, 60000, pendingIntent")
		if len(findings) != 1 || findings[0].Message != lintMessage("5000") || findings[0].Col != 49 {
			t.Fatalf("expected one trigger finding at col 49, got %+v", findings)
		}
	})
	t.Run("both short report both", func(t *testing.T) {
		if got := len(run("AlarmManager.RTC, 1000L, 30000, pendingIntent")); got != 2 {
			t.Fatalf("expected 2 findings, got %d", got)
		}
	})
	t.Run("zero and negative values are reported", func(t *testing.T) {
		if got := len(run("AlarmManager.RTC, 0, 0, pendingIntent")); got != 2 {
			t.Fatalf("expected 2 findings for zeros, got %d", got)
		}
		if got := len(run("AlarmManager.RTC, -1L, 60000, pendingIntent")); got != 1 {
			t.Fatalf("expected 1 finding for a negative trigger, got %d", got)
		}
	})
	t.Run("clean", func(t *testing.T) {
		if got := len(run("AlarmManager.RTC, 5000L, 60000, pendingIntent")); got != 0 {
			t.Fatalf("expected 0 findings, got %d", got)
		}
	})
	t.Run("non-constant values are not reported", func(t *testing.T) {
		if got := len(run("AlarmManager.RTC, time, interval, pendingIntent")); got != 0 {
			t.Fatalf("expected 0 findings, got %d", got)
		}
	})
	t.Run("setInexactRepeating is not checked", func(t *testing.T) {
		findings := runRuleByName(t, "ShortAlarm", `
package test
import android.app.AlarmManager
fun schedule(alarmManager: AlarmManager) {
    alarmManager.setInexactRepeating(AlarmManager.RTC, 0L, 1000L, null)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
	t.Run("other overloads are not checked", func(t *testing.T) {
		if got := len(run("AlarmManager.RTC, 0L, 1000L")); got != 0 {
			t.Fatalf("expected 0 findings for a three-argument call, got %d", got)
		}
	})
	t.Run("named arguments", func(t *testing.T) {
		if got := len(run("type = AlarmManager.RTC, intervalMillis = 1000L, triggerAtMillis = 9000L, operation = pendingIntent")); got != 1 {
			t.Fatalf("expected 1 finding, got %d", got)
		}
	})
}

func TestShortAlarmConstants(t *testing.T) {
	run := func(interval string) int {
		return len(runRuleByName(t, "ShortAlarm", `
package test
import android.app.AlarmManager
fun schedule(alarmManager: AlarmManager) {
    alarmManager.setRepeating(AlarmManager.RTC, 9000L, `+interval+`, null)
}
`))
	}
	for interval, want := range map[string]int{
		"30_000L": 1, "30 * 1000L": 1, "intervalMillis = 1000L": 1, "0x7530": 1, "0L": 1,
		"60_000L": 0, "2 * 60 * 1000L": 0, "AlarmManager.INTERVAL_HOUR": 0,
	} {
		if got := run(interval); got != want {
			t.Errorf("interval %q: expected %d findings, got %d", interval, want, got)
		}
	}
	t.Run("file without AlarmManager is skipped", func(t *testing.T) {
		findings := runRuleByName(t, "ShortAlarm", `
package test
fun schedule(t: Timer) { t.setRepeating(1, 2, 500L, null) }
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLocalSuppress(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "LocalSuppress", `
package test

@SuppressLint("NewAPI_")
fun doStuff() {}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("unknown but unrelated id is left alone", func(t *testing.T) {
		findings := runRuleByName(t, "LocalSuppress", `
package test

@SuppressLint("CompanyCustomCheck")
fun doStuff() {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for third-party id, got %d", len(findings))
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "LocalSuppress", `
package test

@SuppressLint("NewApi")
fun doStuff() {}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLocalSuppressAcceptsRegisteredRuleIDs(t *testing.T) {
	findings := runRuleByName(t, "LocalSuppress", `
package test

@SuppressLint("TrulyRandom", "WrongConstant", "WebViewLayout")
fun doStuff() {}
`)
	if len(findings) != 0 {
		t.Fatalf("registered Android lint rule IDs and aliases are valid, got %d findings", len(findings))
	}
}

func TestPluralsCandidate(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "PluralsCandidate", `
package test

fun display(count: Int) {
    if (count == 1) {
        val msg = getString(R.string.item_single)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "PluralsCandidate", `
package test

fun display(count: Int) {
    val msg = resources.getQuantityString(R.plurals.items, count, count)
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestCustomViewStyleable_OnlyCustomViews(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"mismatched custom view", "class MyView(c: Context) : View(c) { init { c.obtainStyledAttributes(null, R.styleable.Other) } }", 1},
		{"attrs only overload", "class MyView(c: Context) : View(c) { init { c.obtainStyledAttributes(R.styleable.Other) } }", 1},
		{"matching name", "class MyView(c: Context) : View(c) { init { c.obtainStyledAttributes(null, R.styleable.MyView) } }", 0},
		{"activity is not a view", "class Screen : Activity() { fun f() { obtainStyledAttributes(null, R.styleable.Theme) } }", 0},
		{"subclass reuses base styleable", "class MyView(c: Context) : BaseView(c) { init { c.obtainStyledAttributes(null, R.styleable.BaseView) } }", 0},
		{"companion helper", "class MyView(c: Context) : View(c) { companion object { fun f(c: Context) { c.obtainStyledAttributes(null, R.styleable.Zed) } } }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test\nimport android.app.Activity\nimport android.content.Context\nimport android.view.View\n" + tc.body + "\n"
			if got := len(runRuleByName(t, "CustomViewStyleable", src)); got != tc.want {
				t.Fatalf("expected %d findings, got %d", tc.want, got)
			}
		})
	}
}

func TestDefaultLocale_Precision(t *testing.T) {
	t.Run("kotlin", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "DefaultLocale", `
package test
import java.util.Locale
fun f(s: String, c: Char) {
    c.toLowerCase()
    toLowerCase(s)
    s.toLowerCase(Locale.getDefault())
    String.format("%s", s)
    String.format(locale = Locale.US, format = "%d", 1)
    java.lang.String.format("%d", 1)
    s.toUpperCase()
    String.format("%d", 1)
}
`)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings (java.lang.String.format, toUpperCase, format; %%s is locale-neutral), got %v", findings)
		}
	})
	t.Run("java", func(t *testing.T) {
		findings := runJavaRuleByName(t, "DefaultLocale", `
package test;
import java.util.Locale;
class C {
  void f(String s, char c) {
    Character.toLowerCase(c);
    s.toLowerCase(Locale.ROOT);
    String.format("%s", s);
    String.format(Locale.US, "%d", 1);
    java.lang.String.format("%.2f", 1.0);
    s.toUpperCase();
    String.format("%d", 1);
  }
}
`)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %v", findings)
		}
	})
}

func TestDefaultLocale_FormatFilters(t *testing.T) {
	t.Run("java", func(t *testing.T) {
		findings := runJavaRuleByName(t, "DefaultLocale", `
package test;
import android.util.Log;
class C {
  static final String FMT = "%d x";
  void f(String fmt, String s) {
    String.format("%s", s);
    String.format("%d", 1);
    String.format("%5.2f", 1.0);
    String.format("%x", 1);
    String.format("%%", 1);
    String.format("plain");
    String.format(fmt, s);
    String.format(FMT, 1);
    String.format("a" + "%d", 1);
    Log.d("t", String.format("%d", 1));
    throw new IllegalStateException(String.format("%d", 1));
  }
}
`)
		var lines []int
		for _, f := range findings {
			lines = append(lines, f.Line)
		}
		want := []int{8, 9, 14, 15}
		if len(lines) != len(want) {
			t.Fatalf("expected findings on lines %v, got %v", want, lines)
		}
		for i := range want {
			if lines[i] != want[i] {
				t.Fatalf("expected findings on lines %v, got %v", want, lines)
			}
		}
	})
	t.Run("kotlin", func(t *testing.T) {
		findings := runRuleByNameWithResolver(t, "DefaultLocale", `
package test
import android.util.Log
const val FMT = "%d x"
fun f(fmt: String, s: String) {
    String.format("%s", s)
    String.format("%d", 1)
    String.format("plain")
    String.format(fmt, s)
    String.format(FMT, 1)
    String.format(format = "%f", 1.0)
    Log.d("t", String.format("%d", 1))
    if (s.isEmpty()) throw IllegalStateException(String.format("%d", 1))
}
`)
		var lines []int
		for _, f := range findings {
			lines = append(lines, f.Line)
		}
		want := []int{7, 10, 11}
		if len(lines) != len(want) {
			t.Fatalf("expected findings on lines %v, got %v", want, lines)
		}
		for i := range want {
			if lines[i] != want[i] {
				t.Fatalf("expected findings on lines %v, got %v", want, lines)
			}
		}
	})
}

func TestObjectAnimatorBinding_SubclassSetters(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ObjectAnimatorBinding", `
package test
import android.animation.ObjectAnimator
import android.view.View
import android.widget.TextView

fun animate(view: View, label: TextView) {
    ObjectAnimator.ofFloat(view, "scrollX", 0f, 1f)
    ObjectAnimator.ofFloat(label, "textSize", 1f, 2f)
}
`)
	if len(findings) != 0 {
		t.Fatalf("scrollX on View and textSize on TextView are real setters, got %d findings", len(findings))
	}
}
