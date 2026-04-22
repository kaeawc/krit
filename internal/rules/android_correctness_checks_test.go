package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/oracle"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

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

func TestTextViewEdits(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "TextViewEdits", `
package test

fun update() {
    editTextField.setText("hello")
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "TextViewEdits", `
package test

fun update() {
    textView.setText("hello")
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
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
	file, err := scanner.ParseJavaFile(path)
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
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
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
	for i := range file.FlatTree.Nodes {
		flatIdx := uint32(i)
		if !wants[file.FlatType(flatIdx)] {
			continue
		}
		node := file.FlatTree.Nodes[i]
		rule.Check(&v2rules.Context{
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
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "SupportAnnotationUsage", `
package test

class MyActivity {
    @MainThread
    fun loadData() {
        val conn = HttpURLConnection()
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "SupportAnnotationUsage", `
package test

class MyActivity {
    @MainThread
    fun updateUI() {
        textView.text = "hello"
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAccidentalOctal(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "AccidentalOctal", `
package test

val perms = 0755
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "AccidentalOctal", `
package test

val perms = 493
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAppCompatMethod(t *testing.T) {
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "AppCompatMethod", `
package test

fun setup() {
    getActionBar()
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
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
	t.Run("triggers", func(t *testing.T) {
		findings := runRuleByName(t, "ShortAlarm", `
package test

fun schedule() {
    alarmManager.setRepeating(AlarmManager.RTC, time, 5000, pendingIntent)
}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
		}
	})
	t.Run("clean", func(t *testing.T) {
		findings := runRuleByName(t, "ShortAlarm", `
package test

fun schedule() {
    alarmManager.setRepeating(AlarmManager.RTC, time, 60000, pendingIntent)
}
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

@SuppressLint("NotARealIssue")
fun doStuff() {}
`)
		if len(findings) == 0 {
			t.Fatal("expected findings")
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
