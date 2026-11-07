package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestWrongImportRule(t *testing.T) {
	t.Run("flags import android.R", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R
class Foo
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for import android.R")
		}
	})

	t.Run("unused import android.R.drawable is left to UnusedImport", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.drawable
class Foo
`)
		if len(findings) != 0 {
			t.Fatalf("expected no finding for an unused sub-class import, got %v", findings)
		}
	})

	t.Run("framework attr id used as a value says so", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.attr.minHeight
fun size() = minHeight + 1
`)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		msg := findings[0].Message
		if strings.Contains(msg, "not referenced by name") || !strings.Contains(msg, "framework R resource id") {
			t.Fatalf("unexpected message: %s", msg)
		}
	})

	t.Run("unused framework attr import is left to UnusedImport", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.attr.minHeight
class Foo
`)
		if len(findings) != 0 {
			t.Fatalf("an unused import is not a wrong-import value use, got %v", findings)
		}
	})

	t.Run("qualified selectors, labels, parameters and docs are not uses", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.attr.subtitle
import android.R.attr.colorPrimary

/** Shows the subtitle and the colorPrimary. */
class Foo(val item: Item) {
  fun show(colorPrimary: Int) {
    render(subtitle = item.subtitle, tint = colorPrimary)
    println(item.subtitle)
  }
  fun other() {
    val subtitle = "x"
    println(subtitle)
  }
}
`)
		if len(findings) != 0 {
			t.Fatalf("no reference resolves to the imported ids, got %v", findings)
		}
	})

	t.Run("bare identifier next to a shadowing parameter in another function still resolves", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.attr.subtitle

fun shadowed(subtitle: Int) = subtitle
fun real() = subtitle + 1
`)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "used as a value") {
			t.Fatalf("expected the value-use finding, got %v", findings)
		}
	})

	t.Run("ignores aliased android.R import", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R as AndroidR
class Foo {
    val id = AndroidR.id.content
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for aliased android.R, got %d", len(findings))
		}
	})

	t.Run("ignores android.R sub-class import that is used", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.attr
class Foo {
    val id = attr.selectableItemBackground
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for used android.R.attr, got %d", len(findings))
		}
	})

	t.Run("ignores import com.example.R", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import com.example.R
class Foo
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for com.example.R, got %d", len(findings))
		}
	})

	t.Run("ignores file without android.R import", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.os.Bundle
class Foo
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %d", len(findings))
		}
	})
}

func TestGetSignaturesRule(t *testing.T) {
	t.Run("flags GET_SIGNATURES in getPackageInfo flags", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNATURES)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for GET_SIGNATURES")
		}
	})

	t.Run("flags local GET_SIGNATURES flag variable passed to getPackageInfo", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check(pm: PackageManager) {
        val flags = PackageManager.GET_SIGNATURES
        pm.getPackageInfo("com.example", flags)
    }
}
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for GET_SIGNATURES")
		}
	})

	t.Run("ignores GET_SIGNING_CERTIFICATES", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check(pm: PackageManager) {
        pm.getPackageInfo("com.example", PackageManager.GET_SIGNING_CERTIFICATES)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for GET_SIGNING_CERTIFICATES, got %d", len(findings))
		}
	})

	t.Run("ignores file without either constant", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun doStuff() {
        println("hello")
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %d", len(findings))
		}
	})

	t.Run("ignores GET_SIGNATURES inside Build.VERSION.SDK_INT guarded branch", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check(pm: PackageManager, name: String) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            pm.getPackageInfo(name, PackageManager.GET_SIGNING_CERTIFICATES)
        } else {
            pm.getPackageInfo(name, PackageManager.GET_SIGNATURES)
        }
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for SDK-guarded GET_SIGNATURES, got %d", len(findings))
		}
	})

	t.Run("ignores incidental GET_SIGNATURES constant not used as package info flag", func(t *testing.T) {
		findings := runRuleByName(t, "GetSignatures", `package test
class Foo {
    fun check() {
        val flags = PackageManager.GET_SIGNATURES
        println(flags)
    }
}
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings for incidental constant, got %d", len(findings))
		}
	})
}

func TestLayoutInflationRule(t *testing.T) {
	t.Run("anchors at the null parent argument like Lint", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun a(context: Context): View {
        return LayoutInflater.from(context)
            .inflate(R.layout.with_root_layout_params, null, false)
    }

    fun b(inflater: LayoutInflater): View {
        val v = inflater.inflate(R.layout.with_root_layout_params, null)
        return v
    }
}
`, layoutInflationTestIndex())
		want := [][2]int{{5, 56}, {9, 68}}
		if len(findings) != len(want) {
			t.Fatalf("got %d findings, want %d: %v", len(findings), len(want), findings)
		}
		for i, f := range findings {
			if f.Line != want[i][0] || f.Col != want[i][1] {
				t.Errorf("finding %d at %d:%d, want %d:%d", i, f.Line, f.Col, want[i][0], want[i][1])
			}
		}
	})

	t.Run("flags inflate with null parent when root layout params exist", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun onCreateView(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.with_root_layout_params, null)
    }
}
`, layoutInflationTestIndex())
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null parent")
		}
	})

	t.Run("ignores inflate whose result is discarded", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun warmUp(inflater: LayoutInflater) {
        inflater.inflate(R.layout.with_root_layout_params, null)
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for an unused inflate result, got %d", len(findings))
		}
	})

	t.Run("ignores inflate in test source sets", func(t *testing.T) {
		file := parseInline(t, `package test
class FooTest {
    fun inflateIt(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.with_root_layout_params, null)
    }
}
`)
		file.Path = "/repo/app/src/test/java/test/FooTest.kt"
		for _, r := range api.Registry {
			if r.ID == "LayoutInflation" {
				dispatcher := rules.NewDispatcher([]*api.Rule{r}, nil)
				cols := dispatcher.RunResourceSource(file, layoutInflationTestIndex())
				if got := cols.Findings(); len(got) != 0 {
					t.Fatalf("expected no findings in test sources, got %d", len(got))
				}
				return
			}
		}
		t.Fatal("LayoutInflation not found in registry")
	})

	t.Run("flags multiline chained inflate", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun onCreateView(context: Context): View {
        return LayoutInflater
            .from(context)
            .inflate(
                R.layout.with_root_layout_params,
                null
            )
    }
}
`, layoutInflationTestIndex())
		if len(findings) == 0 {
			t.Fatal("expected finding for multiline inflate with null parent")
		}
	})

	t.Run("ignores layout without root layout params", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun create(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.no_root_layout_params, null)
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings without root layout params, got %d", len(findings))
		}
	})

	t.Run("dialog context in another function does not suppress inflate", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun buildDialog() {
        AlertDialog.Builder(context).setView(View(context))
    }
    fun onCreateView(inflater: LayoutInflater): View {
        return inflater.inflate(R.layout.with_root_layout_params, null)
    }
}
`, layoutInflationTestIndex())
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null parent when dialog context is only in another function")
		}
	})

	t.Run("ignores inflate with proper parent", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun onCreateView(inflater: LayoutInflater, container: ViewGroup?): View {
        return inflater.inflate(R.layout.with_root_layout_params, container, false)
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for inflate with parent, got %d", len(findings))
		}
	})

	t.Run("ignores dialog builder view inflation", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun dialog(inflater: LayoutInflater): Dialog {
        val view = inflater.inflate(R.layout.with_root_layout_params, null)
        return AlertDialog.Builder(context).setView(view).create()
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for dialog inflation, got %d", len(findings))
		}
	})

	t.Run("ignores popup window inflation", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun popup(inflater: LayoutInflater): PopupWindow {
        return PopupWindow(inflater.inflate(R.layout.with_root_layout_params, null))
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for popup inflation, got %d", len(findings))
		}
	})

	t.Run("ignores offscreen bitmap rendering", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class ReceiptImageRenderer {
    fun render(inflater: LayoutInflater): Bitmap {
        val view = inflater.inflate(R.layout.with_root_layout_params, null)
        val bitmap = Bitmap.createBitmap(100, 100, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        view.draw(canvas)
        return bitmap
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for offscreen rendering, got %d", len(findings))
		}
	})

	t.Run("flags inflate when non-null ViewGroup parameter is in scope", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class ItemAdapter {
    fun onCreateViewHolder(parent: ViewGroup, viewType: Int): ViewHolder {
        val view = LayoutInflater.from(parent.context).inflate(R.layout.unknown_layout, null)
        return ViewHolder(view)
    }
}
`, layoutInflationTestIndex())
		if len(findings) == 0 {
			t.Fatal("expected finding for inflate with null when ViewGroup parent is in scope")
		}
	})

	t.Run("does not flag when only nullable ViewGroup parameter is in scope", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun create(inflater: LayoutInflater, container: ViewGroup?): View {
        return inflater.inflate(R.layout.unknown_layout, null)
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings when only nullable ViewGroup is in scope, got %d", len(findings))
		}
	})

	t.Run("ignores Compose AndroidView factory", func(t *testing.T) {
		findings := runLayoutInflationRule(t, `package test
class Foo {
    fun Content() {
        AndroidView(
            factory = {
                LayoutInflater.from(it).inflate(R.layout.with_root_layout_params, null)
            },
            modifier = Modifier.fillMaxWidth()
        )
    }
}
`, layoutInflationTestIndex())
		if len(findings) != 0 {
			t.Fatalf("expected no findings for AndroidView factory, got %d", len(findings))
		}
	})
}

func runLayoutInflationRule(t *testing.T, code string, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	for _, r := range api.Registry {
		if r.ID == "LayoutInflation" {
			dispatcher := rules.NewDispatcher([]*api.Rule{r}, nil)
			cols := dispatcher.RunResourceSource(file, idx)
			return cols.Findings()
		}
	}
	t.Fatal("LayoutInflation not found in registry")
	return nil
}

func layoutInflationTestIndex() *android.ResourceIndex {
	idx := emptyIndex()
	add := func(name string, root *android.View) {
		layout := &android.Layout{Name: name, FilePath: "res/layout/" + name + ".xml", RootView: root}
		idx.Layouts[name] = layout
		idx.LayoutConfigs[name] = map[string]*android.Layout{"": layout}
	}
	add("main", &android.View{
		Type: "TextView",
		Attributes: map[string]string{
			"android:layout_width":  "match_parent",
			"android:layout_height": "wrap_content",
		},
	})
	add("with_root_layout_params", &android.View{
		Type: "TextView",
		Attributes: map[string]string{
			"android:layout_width":  "match_parent",
			"android:layout_height": "wrap_content",
		},
	})
	add("no_root_layout_params", &android.View{
		Type:       "TextView",
		Attributes: map[string]string{"android:text": "@string/title"},
	})
	return idx
}
