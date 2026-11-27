package rules_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
)

// The override's class is found across files, through the declaring file's imports: a `View` imported from another
// package there is not android.view.View.
func TestDoNotCallViewToStringCrossFileHierarchy(t *testing.T) {
	views := `package com.example.views

import android.content.Context
import android.view.View

open class BadgeView(context: Context) : View(context) {
  override fun toString(): String = "BadgeView"
}
`
	lookalike := `package com.example.fake

class View {
  open override fun toString(): String = "fake"
}

class FakeView : View() {
  override fun toString(): String = "FakeView"
}
`
	caller := `package com.example

import com.example.fake.FakeView
import com.example.views.BadgeView

fun log(badge: BadgeView, fake: FakeView, nullable: BadgeView?) {
  badge.toString()
  fake.toString()
  nullable?.toString()
  nullable.toString()
}
`
	got := runRuleOnSourcesWithResolver(t, "DoNotCallViewToString", map[string]string{
		"views/BadgeView.kt": views, "fake/Fake.kt": lookalike, "Caller.kt": caller,
	}, "Caller.kt")
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	if fmt.Sprint(lines) != "[7 9]" {
		t.Errorf("want findings on badge.toString() and nullable?.toString(), got %v", got)
	}
}

// An inflate() in a class nested in an ItemDecoration, or on an unrelated receiver, is not reported; one in an
// anonymous ItemDecoration is.
func TestInflationInItemDecorationScopes(t *testing.T) {
	const src = `package com.example

import android.view.LayoutInflater
import android.view.ViewGroup
import androidx.recyclerview.widget.RecyclerView.ItemDecoration

class Inflater { fun inflate(id: Int) = id }

val decoration = object : ItemDecoration() {
  fun draw(parent: ViewGroup, own: Inflater) {
    LayoutInflater.from(parent.context).inflate(1, parent, false)
    own.inflate(1)
  }
}

class Outer : ItemDecoration() {
  class Helper(private val inflater: LayoutInflater) {
    fun make() = inflater.inflate(1, null)
  }
}
`
	got := runRuleOnSourcesWithResolver(t, "InflationInItemDecoration", map[string]string{"Deco.kt": src}, "Deco.kt")
	if len(got) != 1 || got[0].Line != 11 {
		t.Errorf("want one finding, in the anonymous ItemDecoration, got %v", got)
	}
}

// CodeRabbit on #1829: an overload toString(radix) is not the override a zero-argument call resolves to, a spaced
// call passes the gate, and a source class in a databinding package that does not implement ViewBinding is not one.
func TestViewChecksReviewEdgeCases(t *testing.T) {
	got := runRuleOnSourcesWithResolver(t, "DoNotCallViewToString", map[string]string{"V.kt": `package com.example

import android.content.Context
import android.view.View

class RadixView(c: Context) : View(c) {
  fun toString(radix: Int): String = radix.toString()
}

class Named(c: Context) : View(c) {
  override fun toString(): String = "named"
}

fun f(r: RadixView, n: Named) {
  r.toString()
  n.toString ()
}
`}, "V.kt")
	if len(got) != 1 || got[0].Line != 16 {
		t.Errorf("want only the spaced call on the real override (line 16), got %v", got)
	}
	got = runRuleOnSourcesWithResolver(t, "InflationInItemDecoration", map[string]string{
		"DebugBinding.kt": "package com.example.databinding\n\nclass DebugBinding {\n  companion object {\n    fun inflate(x: Any?) = DebugBinding()\n  }\n}\n",
		"Deco.kt": `package com.example

import androidx.recyclerview.widget.RecyclerView
import com.example.databinding.DebugBinding

class Deco : RecyclerView.ItemDecoration() {
  fun draw() = DebugBinding.inflate(null)
}
`}, "Deco.kt")
	if len(got) != 0 {
		t.Errorf("a source class that is not a ViewBinding: want no finding, got %v", got)
	}
}

// Orchestrator first pass on #1829: an untyped inflater property, a library ItemDecoration subclass, a companion
// inside a decoration, a Java source superclass, and toString() in a plain lambda versus an apply block.
func TestViewChecksFirstPassReview(t *testing.T) {
	got := runRuleOnSourcesWithResolver(t, "InflationInItemDecoration", map[string]string{"D.kt": `package com.example

import android.content.Context
import android.view.LayoutInflater
import android.view.ViewGroup
import androidx.recyclerview.widget.DividerItemDecoration
import androidx.recyclerview.widget.RecyclerView

class Header(parent: ViewGroup) : RecyclerView.ItemDecoration() {
  private val inflater = LayoutInflater.from(parent.context)
  fun draw(p: ViewGroup) = inflater.inflate(0, p, false)

  companion object {
    fun make(i: LayoutInflater) = i.inflate(0, null)
  }
}

class Divider(c: Context, private val i: LayoutInflater) : DividerItemDecoration(c, 0) {
  fun draw() = i.inflate(0, null)
}
`}, "D.kt")
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	if fmt.Sprint(lines) != "[11 19]" {
		t.Errorf("InflationInItemDecoration: want lines 11 and 19, got %v", got)
	}
	got = runRuleOnSourcesWithResolverAll(t, "DoNotCallViewToString", map[string]string{
		"KView.kt": `package com.example

import android.content.Context
import android.widget.LinearLayout

class KView(c: Context) : LinearLayout(c) {
  override fun toString(): String = "k"
  fun log() {
    post { toString() }
    StringBuilder().apply { toString() }
  }
}

fun f(k: KView) = k.toString()
`})
	lines = nil
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	if fmt.Sprint(lines) != "[9 14]" {
		t.Errorf("DoNotCallViewToString: want lines 9 and 14, got %v", got)
	}
	// A Kotlin View whose superclass is a Java source class: the resolver indexes Java classes, so k.toString()
	// is reported (#1885).
	got = runRuleOnSourcesWithResolverAll(t, "DoNotCallViewToString", map[string]string{
		"JBase.java": "package com.example;\n\nimport android.content.Context;\nimport android.widget.LinearLayout;\n\npublic class JBase extends LinearLayout {\n  public JBase(Context c) { super(c); }\n}\n",
		"KView.kt":   "package com.example\n\nimport android.content.Context\n\nclass KView(c: Context) : JBase(c) {\n  override fun toString(): String = \"k\"\n}\n\nfun f(k: KView) = k.toString()\n",
	})
	if len(got) != 1 || got[0].Line != 9 {
		t.Errorf("Java source superclass (#1885): want k.toString() on line 9, got %v", got)
	}
}

// CodeRabbit on #1884: an extension function's receiver, not the enclosing class, is what an unqualified
// toString() inside it calls; and an instance inflate() declared in a source ViewBinding class counts, as in Lint.
func TestViewChecksExtensionReceiverAndInstanceBindingInflate(t *testing.T) {
	got := runRuleOnSourcesWithResolverAll(t, "DoNotCallViewToString", map[string]string{"KView.kt": `package com.example

import android.content.Context
import android.widget.LinearLayout

class KView(c: Context) : LinearLayout(c) {
  override fun toString(): String = "k"
  fun String.render() = toString()
  fun String.renderThis() = this.toString()
  fun plain() = toString()
}

fun KView.describe() = toString()
fun String.other() = toString()
fun KView?.orNull() = toString()
fun List<KView>.list() = toString()
fun Map<String, KView>.map() = toString()
fun Array<KView>.array() = toString()
fun List<KView>?.nullableList() = toString()
fun com.example.KView.qualified() = toString()
`})
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	// A View inside the receiver's type arguments doesn't make the receiver a View (#1926).
	if fmt.Sprint(lines) != "[10 13 20]" {
		t.Errorf("DoNotCallViewToString: want lines 10, 13 and 20, got %v", got)
	}
	got = runRuleOnSourcesWithResolverAll(t, "InflationInItemDecoration", map[string]string{"Deco.kt": `package com.example

import android.view.View
import androidx.recyclerview.widget.RecyclerView
import androidx.viewbinding.ViewBinding

class ManualBinding(private val v: View) : ViewBinding {
  override fun getRoot(): View = v
  fun inflate(): View = v
}

class Other {
  fun inflate() {}
}

class Deco(private val binding: ManualBinding, private val other: Other, private val plain: PlainBinding) : RecyclerView.ItemDecoration() {
  fun draw() {
    binding.inflate()
    other.inflate()
    plain.inflate()
  }
}

// An extension inflate() is the file facade's, not the ViewBinding's (#1894 review, P3).
class PlainBinding(private val v: View) : ViewBinding {
  override fun getRoot(): View = v
}

fun PlainBinding.inflate(): View = root
`})
	if len(got) != 1 || got[0].Line != 18 {
		t.Errorf("InflationInItemDecoration: want one finding, on binding.inflate(), got %v", got)
	}
}

// #1949: a function-type receiver inside a View class is the function's, not the class's, so its toString() is not
// reported; a typealias, parenthesized or bounded type-parameter receiver naming a View is.
func TestViewChecksExtensionReceiverShapes(t *testing.T) {
	got := runRuleOnSourcesWithResolverAll(t, "DoNotCallViewToString", map[string]string{"KView.kt": `package com.example

import android.content.Context
import android.widget.LinearLayout

class KView(c: Context) : LinearLayout(c) {
  override fun toString(): String = "k"
  fun (() -> Unit).fnType() = toString()
  fun ((Int) -> KView).fnTypeReturningView() = toString()
}

typealias KV = KView
fun KV.alias() = toString()
fun (KView).paren() = toString()
fun <T : KView> T.bounded() = toString()
fun <T> T.whereBounded() where T : KView = toString()
fun <T> T.unbounded() = toString()
fun (KView)?.parenNullable() = toString()
`})
	var lines []int
	for _, f := range got {
		lines = append(lines, f.Line)
	}
	sort.Ints(lines)
	if fmt.Sprint(lines) != "[13 14 15 16]" {
		t.Errorf("DoNotCallViewToString: want lines 13-16, got %v", got)
	}
}

// #1953 review: a nullable receiver written through a typealias, a nullable bound, or parentheses is Any?'s
// toString(), not the View's; every upper bound of a type parameter counts, a typealias bound resolves, and an alias
// imported from another package resolves.
func TestViewChecksExtensionReceiverNullabilityAndBounds(t *testing.T) {
	got := runRuleOnSourcesWithResolverAll(t, "DoNotCallViewToString", map[string]string{
		"KView.kt": `package com.example

import android.content.Context
import android.widget.LinearLayout

class KView(c: Context) : LinearLayout(c) {
  override fun toString(): String = "k"
}

typealias KVN = KView?
typealias KV = KView
fun KVN.nullableAlias() = toString()
fun <T : KView?> T.nullableBound() = toString()
fun (KView?).parenNullable() = toString()
fun <T> T.secondWhereBound() where T : Runnable, T : KView = toString()
fun <T : KV> T.aliasBound() = toString()
`,
		"Other.kt": `package com.other

import com.example.KV

fun KV.importedAlias() = toString()
`,
	})
	var lines []string
	for _, f := range got {
		lines = append(lines, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
	}
	sort.Strings(lines)
	if want := "[KView.kt:15 KView.kt:16 Other.kt:5]"; fmt.Sprint(lines) != want {
		t.Errorf("DoNotCallViewToString: want %s, got %v", want, got)
	}
}
