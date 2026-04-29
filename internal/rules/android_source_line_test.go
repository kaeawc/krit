package rules_test

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
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

	t.Run("flags import android.R.drawable", func(t *testing.T) {
		findings := runRuleByName(t, "WrongImport", `package test
import android.R.drawable
class Foo
`)
		if len(findings) == 0 {
			t.Fatal("expected finding for import android.R.drawable")
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
	for _, r := range v2rules.Registry {
		if r.ID == "LayoutInflation" {
			dispatcher := rules.NewDispatcherV2([]*v2rules.Rule{r})
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
