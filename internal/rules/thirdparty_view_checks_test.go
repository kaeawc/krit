package rules_test

import (
	"fmt"
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
	// A Kotlin View whose superclass is a Java source class is a known false negative: the pipeline indexes only
	// Kotlin files into the source resolver, so the Java class's supertypes are unknown (#1885). When #1885 lands,
	// this case should report k.toString().
	got = runRuleOnSourcesWithResolverAll(t, "DoNotCallViewToString", map[string]string{
		"JBase.java": "package com.example;\n\nimport android.content.Context;\nimport android.widget.LinearLayout;\n\npublic class JBase extends LinearLayout {\n  public JBase(Context c) { super(c); }\n}\n",
		"KView.kt":   "package com.example\n\nimport android.content.Context\n\nclass KView(c: Context) : JBase(c) {\n  override fun toString(): String = \"k\"\n}\n\nfun f(k: KView) = k.toString()\n",
	})
	if len(got) != 0 {
		t.Errorf("Java source superclass (#1885): the pipeline can't see it yet; got %v", got)
	}
}
