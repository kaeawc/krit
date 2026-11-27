package rules_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func findingFileLines(got []scanner.Finding) []string {
	var out []string
	for _, f := range got {
		out = append(out, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
	}
	sort.Strings(out)
	return out
}

// #1857: a Kotlin Resources subclass, direct or indirect and in any file, or a Context subclass, is a Resources
// receiver or owner. The resolver records supertypes as written (`Resources`, `ContextWrapper`), so each must be
// resolved through the declaring file's imports before it is compared with the FQNs.
func TestArgInFormattedQuantityStringResResourcesSubclasses(t *testing.T) {
	got := runRuleOnSourcesWithResolverAll(t, "ArgInFormattedQuantityStringRes", map[string]string{
		"A.kt": `package com.example

import android.content.Context
import android.content.ContextWrapper
import android.content.res.AssetManager
import android.content.res.Configuration
import android.content.res.Resources
import android.util.DisplayMetrics

open class MyRes(a: AssetManager, d: DisplayMetrics, c: Configuration) : Resources(a, d, c)
class SubRes(a: AssetManager, d: DisplayMetrics, c: Configuration) : MyRes(a, d, c)
class MyCtx(base: Context) : ContextWrapper(base)

fun sub(r: MyRes, n: Int) = r.getQuantityString(1, n, n)
fun subSub(r: SubRes, n: Int) = r.getQuantityString(1, n, n)
fun java(r: JRes, n: Int) = r.getQuantityString(1, n, n)
fun owner(c: MyCtx, n: Int) = c.resources.getQuantityString(1, n, n)
`,
		"B.kt": `package com.example

class OtherFile(private val r: SubRes) {
  fun f(n: Int) = r.getQuantityString(1, n, n)
}
`,
		"JRes.java": `package com.example;

import android.content.res.AssetManager;
import android.content.res.Configuration;
import android.content.res.Resources;
import android.util.DisplayMetrics;

public class JRes extends Resources {
  public JRes(AssetManager a, DisplayMetrics d, Configuration c) { super(a, d, c); }

  String f(JRes r, int n) { return r.getQuantityString(1, n, n); }
}
`,
	})
	// java() on A.kt:16 and JRes.f see JRes's supertypes: the resolver indexes Java classes (#1885).
	want := "[A.kt:14 A.kt:15 A.kt:16 A.kt:17 B.kt:4 JRes.java:11]"
	if got := fmt.Sprint(findingFileLines(got)); got != want {
		t.Errorf("Resources subclasses: want %s, got %s", want, got)
	}
}

// A project class named Resources, and a subclass of it, are lookalikes: no finding.
func TestArgInFormattedQuantityStringResResourcesLookalikeSubclass(t *testing.T) {
	got := runRuleOnSourcesWithResolverAll(t, "ArgInFormattedQuantityStringRes", map[string]string{
		"Res.kt": `package com.example.res

open class Resources {
  fun getQuantityString(id: Int, n: Int, vararg args: Any): String = ""
}
`,
		"A.kt": `package com.example

import com.example.res.Resources

class MyRes : Resources()
open class Wrapper
class MyCtx : Wrapper() {
  val resources = MyRes()
}

fun sub(r: MyRes, n: Int) = r.getQuantityString(1, n, n)
fun owner(c: MyCtx, n: Int) = c.resources.getQuantityString(1, n, n)
`,
		"JRes.java": `package com.example;

import com.example.res.Resources;

public class JRes extends Resources {
  String f(JRes r, int n) { return r.getQuantityString(1, n, n); }
}
`,
	})
	if len(got) != 0 {
		t.Errorf("lookalike Resources subclass: want no finding, got %v", findingFileLines(got))
	}
}
