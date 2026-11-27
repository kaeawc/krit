package rules_test

import (
	"strings"
	"testing"
)

// The parity case (testdata/slack_lint/denylist) runs without a resolver; these cover what the source resolver
// adds and the lookalikes the case does not.

// A View subclass declared in source inherits View.setOnClickListener; a local class named View does not.
func TestDenyListedApiViewSubtypeThroughResolver(t *testing.T) {
	const src = `package com.example

import android.content.Context
import android.view.View

class MyButton(context: Context) : View(context)

fun f(button: MyButton) {
  button.setOnClickListener(null)
}
`
	if got := runRuleWithSourceResolver(t, "DenyListedApi", src); len(got) != 1 {
		t.Errorf("want one finding on a View subclass, got %v", got)
	}
	const lookalike = `package com.example

class View {
  fun setOnClickListener(listener: Any?) {}
}

fun f(view: View) {
  view.setOnClickListener(null)
}
`
	if got := runRuleWithSourceResolver(t, "DenyListedApi", lookalike); len(got) != 0 {
		t.Errorf("want no finding on a local View class, got %v", got)
	}
}

// RxJava 3 subjects inherit the base type's blocking methods, with that type's message.
func TestDenyListedBlockingApiSubjectUsesBaseTypeMessage(t *testing.T) {
	const src = `package com.example

import io.reactivex.rxjava3.subjects.CompletableSubject
import io.reactivex.rxjava3.subjects.SingleSubject

fun f(single: SingleSubject<Int>, completable: CompletableSubject) {
  single.blockingGet()
  completable.blockingAwait()
}
`
	got := runRuleByName(t, "DenyListedBlockingApi", src)
	if len(got) != 2 {
		t.Fatalf("want two findings, got %v", got)
	}
	if !strings.Contains(got[0].Message, "return this Single,") || !strings.Contains(got[1].Message, "return this Completable, a Disposable to grant control to the caller, or") {
		t.Errorf("want the Single and Completable messages, got %q and %q", got[0].Message, got[1].Message)
	}
}

// A receiver whose type the source does not show abstains, and so does an RxJava 2 type.
func TestDenyListedBlockingApiNeedsRxJava3Receiver(t *testing.T) {
	const src = `package com.example

import io.reactivex.Single
import io.reactivex.rxjava3.core.Observable

fun f(old: Single<Int>, obs: Observable<Int>) {
  old.blockingGet()
  load().blockingGet()
  obs.blockingFirst()
}
`
	if got := runRuleByName(t, "DenyListedBlockingApi", src); len(got) != 1 || got[0].Line != 9 {
		t.Errorf("want one finding, on obs.blockingFirst(), got %v", got)
	}
}

// Kotlin resolves a same-scope function ahead of kotlin.runCatching and an imported top-level function.
func TestDenyListedApiShadowedFunctions(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"stdlib runCatching", "fun f() {\n  runCatching { 1 }\n}\n", 1},
		{"member runCatching", "class C {\n  fun <T> runCatching(block: () -> T) = block()\n  fun f() {\n    runCatching { 1 }\n  }\n}\n", 0},
		{"local runCatching", "fun f() {\n  fun <T> runCatching(block: () -> T) = block()\n  runCatching { 1 }\n}\n", 0},
		{"rxSingle without an import", "fun <T> rxSingle(block: () -> T) = block()\n\nfun f() {\n  rxSingle { 1 }\n}\n", 0},
		{"imported rxSingle", "import kotlinx.coroutines.rx3.rxSingle\n\nfun f() {\n  rxSingle { 1 }\n}\n", 1},
		{"rxSingle with a dispatcher", "import kotlinx.coroutines.Dispatchers\nimport kotlinx.coroutines.rx3.rxSingle\n\nfun f() {\n  rxSingle(Dispatchers.IO) { 1 }\n}\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runRuleByName(t, "DenyListedApi", "package com.example\n\n"+tc.src); len(got) != tc.want {
				t.Errorf("want %d findings, got %v", tc.want, got)
			}
		})
	}
}

// Classes named like the listed ones but not imported from java.util or java.text are not them, in Kotlin or Java.
func TestDenyListedApiLocalLookalikes(t *testing.T) {
	const kotlin = `package com.example

class Date {
  val time = 0L
  fun getTime() = time
}

object Calendar {
  const val YEAR = 1
  fun getInstance() = this
}

fun f() {
  Date().time
  Date().getTime()
  Calendar.getInstance()
  Calendar.YEAR
}
`
	if got := runRuleByName(t, "DenyListedApi", kotlin); len(got) != 0 {
		t.Errorf("Kotlin: want no finding, got %v", got)
	}
	const java = `package com.example;

class Calendar {
  static final int YEAR = 1;
  static Calendar getInstance() { return new Calendar(); }
}

class LinkedList {}

class J {
  void f() {
    Calendar.getInstance();
    int y = Calendar.YEAR;
    new LinkedList();
  }
}
`
	if got := runRuleByNameOnJava(t, "DenyListedApi", java); len(got) != 0 {
		t.Errorf("Java: want no finding, got %v", got)
	}
}

// Names in comments and strings are not references.
func TestDenyListedApiIgnoresCommentsAndStrings(t *testing.T) {
	const src = `package com.example

import java.util.Calendar

fun f(): String {
  // Calendar.getInstance() and Calendar.YEAR
  return "Calendar.getInstance() Calendar.YEAR"
}
`
	if got := runRuleByName(t, "DenyListedApi", src); len(got) != 0 {
		t.Errorf("want no finding, got %v", got)
	}
}

// Lint matches the class that declares the resolved method: a Kotlin extension or scope function on a Date is not
// one of Date's methods, while a DateFormat method called on a SimpleDateFormat is.
func TestDenyListedApiOnlyDeclaredMembers(t *testing.T) {
	const src = `package com.example

import java.text.SimpleDateFormat
import java.util.Date

fun f(date: Date?, format: SimpleDateFormat) {
  date?.let { it }
  date?.also { }
  format.format(date)
  format.isLenient
  format.javaClass
}
`
	got := runRuleByName(t, "DenyListedApi", src)
	if len(got) != 2 || got[0].Line != 9 || got[1].Line != 10 {
		t.Errorf("want findings on format.format and format.isLenient only, got %v", got)
	}
}

// CodeRabbit on #1829: a gate word is the bare name, a Java variable named like a listed class is a value, a member
// beats the runCatching extension, an aliased constructor is still the listed class, and an inherited
// blockingGet() without a receiver passes the blocking gate.
func TestDenyListedApiReviewEdgeCases(t *testing.T) {
	const spaced = `package com.example

import io.reactivex.rxjava3.core.Observable

fun f(source: Observable<Int>) = source.hide ()
`
	if got := runRuleByName(t, "DenyListedApi", spaced); len(got) != 1 {
		t.Errorf("spaced hide (): want one finding, got %v", got)
	}
	const javaValue = `package com.example;

import java.util.Date;

class J {
  String use(Object Date) { return Date.toString(); }
}
`
	if got := runRuleByNameOnJava(t, "DenyListedApi", javaValue); len(got) != 0 {
		t.Errorf("a parameter named Date: want no finding, got %v", got)
	}
	const member = `package com.example

class Local {
  fun runCatching(block: () -> Unit) = block()
}

fun f(x: Local) = x.runCatching {}
`
	if got := runRuleWithSourceResolver(t, "DenyListedApi", member); len(got) != 0 {
		t.Errorf("a member runCatching: want no finding, got %v", got)
	}
	const alias = `package com.example

import java.util.LinkedList as LL

fun f() = LL<Int>()
`
	if got := runRuleByName(t, "DenyListedApi", alias); len(got) != 1 {
		t.Errorf("an aliased LinkedList constructor: want one finding, got %v", got)
	}
	const inherited = `package com.example;

import io.reactivex.rxjava3.core.Single;
import io.reactivex.rxjava3.core.SingleObserver;

abstract class Cached extends Single<Integer> {
  Integer now() { return blockingGet(); }
}
`
	if got := runRuleByNameOnJava(t, "DenyListedBlockingApi", inherited); len(got) != 1 {
		t.Errorf("an inherited blockingGet(): want one finding, got %v", got)
	}
}

// Orchestrator first pass on #1819: an untyped subject or relay property, navigation receivers the resolver types,
// a same-package class over a star import, implicit receivers, property writes, and a Java View subclass.
func TestDenyListedApiFirstPassReview(t *testing.T) {
	for _, tc := range []struct {
		name, rule string
		sources    map[string]string
		want       int
	}{
		{"subject property hide()", "DenyListedApi", map[string]string{"S.kt": `package com.example

import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.subjects.BehaviorSubject
import io.reactivex.rxjava3.subjects.PublishSubject
import com.jakewharton.rxrelay3.BehaviorRelay

class Store {
  private val state = BehaviorSubject.createDefault(0)
  private val events = PublishSubject.create<Int>()
  private val relay = BehaviorRelay.create<Int>()
  val a: Observable<Int> get() = state.hide()
  val b: Observable<Int> get() = events.hide()
  val c: Observable<Int> get() = relay.hide()
}
`}, 3},
		{"navigation receiver", "DenyListedApi", map[string]string{"H.kt": `package com.example

import android.content.Context
import android.widget.Button

class Holder(val button: Button)

class Screen(private val holder: Holder, private val buttons: List<Button>) {
  private val button: Button? = null
  fun clear() {
    holder.button.setOnClickListener(null)
    this.button?.setOnClickListener(null)
    buttons[0].setOnClickListener(null)
  }
}
`}, 3},
		{"same-package Date over java.util.*", "DenyListedApi", map[string]string{
			"Date.kt":  "package com.ex\n\nclass Date {\n  fun getTime() = 0L\n}\n\nclass Calendar {\n  companion object {\n    fun getInstance() = Calendar()\n  }\n}\n",
			"Use.kt":   "package com.ex\n\nimport java.util.*\n\nfun f(): Long {\n  val d = Date()\n  Calendar.getInstance()\n  return d.getTime()\n}\n",
			"Use.java": "package com.ex;\n\nimport java.util.*;\n\nclass UseJ {\n  long f() {\n    Date d = new Date();\n    return d.getTime();\n  }\n}\n",
		}, 0},
		{"implicit receivers and a property write", "DenyListedApi", map[string]string{"C.kt": `package com.example

import android.content.Context
import android.view.View
import java.util.Calendar

fun f(millis: Long) {
  val cal = Calendar.getInstance()
  cal.timeInMillis = millis
  Calendar.getInstance().apply { set(Calendar.MONTH, 1) }
}

class Tappable(c: Context) : View(c) {
  init {
    setOnClickListener(null)
  }
}
`}, 6},
		// A known false negative: the pipeline indexes only Kotlin files into the source resolver, so a Kotlin
		// receiver typed as a Java View subclass isn't known to be a View (#1885). When #1885 lands, want 1.
		{"Java View subclass (known false negative, #1885)", "DenyListedApi", map[string]string{
			"JBase.java": "package com.ex;\n\nimport android.content.Context;\nimport android.widget.LinearLayout;\n\npublic class JBase extends LinearLayout {\n  public JBase(Context c) { super(c); }\n}\n",
			"Use.kt":     "package com.ex\n\nfun clear(b: JBase) {\n  b.setOnClickListener(null)\n}\n",
		}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runRuleOnSourcesWithResolverAll(t, tc.rule, tc.sources)
			if len(got) != tc.want {
				t.Errorf("want %d findings, got %v", tc.want, got)
			}
		})
	}
}

// #1888: a member function of a class in the file's package is not a top-level function, so it shadows nothing
// outside its class; an implicit-receiver property write in apply; a constructor property of a class in another file;
// and an indexed Map<String, Button> read.
func TestDenyListedApiPostMergeVerification(t *testing.T) {
	for _, tc := range []struct {
		name, rule string
		sources    map[string]string
		want       int
	}{
		{"same-package member setOnClickListener", "DenyListedApi", map[string]string{
			"Fancy.kt": "package com.ex\n\nimport android.content.Context\nimport android.view.View\nimport android.widget.Button\n\nclass Fancy(c: Context) : Button(c) {\n  override fun setOnClickListener(l: View.OnClickListener?) {}\n}\n",
			"Other.kt": "package com.ex\n\nimport android.content.Context\nimport android.view.View\n\nclass Other(c: Context) : View(c) {\n  init {\n    setOnClickListener(null)\n  }\n}\n",
		}, 1},
		{"same-package member runCatching", "DenyListedApi", map[string]string{
			"Svc.kt": "package com.ex\n\nclass Svc {\n  fun runCatching(block: () -> Int): Int = block()\n}\n",
			"Use.kt": "package com.ex\n\nfun f() = runCatching { 1 }\n",
		}, 1},
		{"same-package top-level runCatching still shadows", "DenyListedApi", map[string]string{
			"Fns.kt": "package com.ex\n\nfun <T> runCatching(block: () -> T): T = block()\n",
			"Use.kt": "package com.ex\n\nfun f() = runCatching { 1 }\n",
		}, 0},
		{"implicit-receiver property write", "DenyListedApi", map[string]string{
			"C.kt": "package com.ex\n\nimport java.util.Calendar\n\nfun f() {\n  Calendar.getInstance().apply { timeInMillis = 5 }\n}\n",
		}, 2},
		{"implicit write shadowed by a local, or on another receiver", "DenyListedApi", map[string]string{
			"C.kt": "package com.ex\n\nimport java.util.Calendar\n\nclass Clock {\n  var timeInMillis = 0L\n}\n\n" +
				"fun f(cal: Calendar) {\n  Clock().apply { timeInMillis = 5 }\n  cal.apply {\n    var timeInMillis = 0L\n    timeInMillis = 5\n  }\n}\n",
		}, 0},
		// A known false negative: the resolver records body members only, so a constructor property of a class in
		// another file has no type here. Recording them would change member lookups for every resolver rule.
		{"constructor property of a class in another file (known false negative)", "DenyListedApi", map[string]string{
			"B.kt":   "package com.ex\n\nimport android.widget.Button\n\nclass Binding(val root: Button)\n",
			"Use.kt": "package com.ex\n\nfun clear(b: Binding) {\n  b.root.setOnClickListener(null)\n}\n",
		}, 0},
		{"indexed Map<String, Button>", "DenyListedApi", map[string]string{
			"Use.kt": "package com.ex\n\nimport android.widget.Button\n\nfun clear(m: Map<String, Button>) {\n  m[\"x\"]?.setOnClickListener(null)\n}\n",
		}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runRuleOnSourcesWithResolverAll(t, tc.rule, tc.sources)
			if len(got) != tc.want {
				t.Errorf("want %d findings, got %v", tc.want, got)
			}
		})
	}
}
