package rules_test

import (
	"strings"
	"testing"
)

// The emit rules follow RxObservableEmitDetectorTest on slack-lints main (c0e92cb74b10), whose detector fixes
// 0.11.1's ProducerScope FQN. Each case is one of its tests; want is the line it reports, or 0 for expectClean.
func TestRxDoesNotEmitMatchesUpstreamMainTests(t *testing.T) {
	for _, factory := range []struct{ name, rule string }{
		{"rxObservable", "RxObservableDoesNotEmit"},
		{"rxFlowable", "RxFlowableDoesNotEmit"},
	} {
		for _, tc := range []struct {
			name string
			body string
			want int
		}{
			{"does not call send or trySend", `$m { println("foo") }`, 7},
			{"calls send", `$m { send("foo") }`, 0},
			{"calls trySend", `$m { trySend("foo") }`, 0},
			{"results of send are not the lambda return value", "$m {\n      send(\"foo\")\n      println(\"bar\")\n    }", 0},
			{"results of trySend are not the lambda return value", "$m {\n      trySend(\"foo\")\n      println(\"bar\")\n    }", 0},
			{"outer factory does not call send", "$m {\n        $m {\n            send(\"foo\")\n        }\n    }", 7},
			{"outer factory does not call trySend", "$m {\n        $m {\n            trySend(\"foo\")\n        }\n    }", 7},
			{"inner factory does not call send", "$m {\n        send(\"foo\")\n        $m {\n          println(\"bar!\")\n        }\n    }", 9},
			{"inner factory does not call trySend", "$m {\n        trySend(\"foo\")\n        $m {\n          println(\"bar!\")\n        }\n    }", 9},
		} {
			t.Run(factory.name+"/"+tc.name, func(t *testing.T) {
				src := "package test\n\nimport kotlinx.coroutines.rx3." + factory.name + "\n\nclass Foo {\n  fun foo() {\n    " +
					strings.ReplaceAll(tc.body, "$m", factory.name) + "\n  }\n}\n"
				got := runRuleByName(t, factory.rule, src)
				switch {
				case tc.want == 0 && len(got) != 0:
					t.Errorf("want clean, got %v", got)
				case tc.want != 0 && (len(got) != 1 || got[0].Line != tc.want || got[0].Message != factory.name+" does not call `send()` or `trySend()`"):
					t.Errorf("want one finding on line %d, got %v", tc.want, got)
				}
			})
		}
	}
}

// SubscribeOnMain needs proof that subscribeOn is RxJava 3's: RxJava 2, RxJava 1 and Reactor declare it too.
func TestSubscribeOnMainRequiresRxJava3(t *testing.T) {
	for name, imports := range map[string]string{
		"RxJava 2": "import io.reactivex.Observable\nimport io.reactivex.android.schedulers.AndroidSchedulers",
		"mixed":    "import io.reactivex.rxjava3.core.Observable\nimport io.reactivex.android.schedulers.AndroidSchedulers",
		"Reactor":  "import reactor.core.publisher.Flux\nimport io.reactivex.rxjava3.android.schedulers.AndroidSchedulers",
		"none":     "",
	} {
		src := "package test\n\n" + imports + "\n\nfun f(obs: Observable<String>) {\n  obs.subscribeOn(AndroidSchedulers.mainThread())\n}\n"
		if got := runRuleByName(t, "SubscribeOnMain", src); len(got) != 0 {
			t.Errorf("%s: want no finding without RxJava 3-only imports, got %v", name, got)
		}
	}
	src := "package test\n\nimport io.reactivex.rxjava3.android.schedulers.AndroidSchedulers\nimport io.reactivex.rxjava3.core.Observable\n\nfun f(obs: Observable<String>) {\n  obs.subscribeOn(AndroidSchedulers.mainThread())\n}\n"
	if got := runRuleByName(t, "SubscribeOnMain", src); len(got) != 1 || got[0].Line != 7 || got[0].Col != 7 {
		t.Errorf("want one finding at 7:7 (the method name), got %v", got)
	}
}

// Review of #1772 (P2): a binding with no initializer shadows an outer mainThread() variable, as Lint resolves the
// name to it.
func TestSubscribeOnMainShadowedVariables(t *testing.T) {
	const kotlin = `package app

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers
import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.core.Scheduler

private val main = AndroidSchedulers.mainThread()

class Repo(private val main: Scheduler) {
  fun load(obs: Observable<String>) = obs.subscribeOn(main)
}

class Loops {
  fun loop(obs: Observable<String>, list: List<Scheduler>, pairs: List<Pair<Scheduler, Int>>) {
    for (main in list) obs.subscribeOn(main)
    for ((main, n) in pairs) obs.subscribeOn(main)
    try { } catch (main: Exception) { obs.subscribeOn(main) }
    when (val main = list.first()) { else -> obs.subscribeOn(main) }
    val (main, other) = pairs.first()
    obs.subscribeOn(main)
    pairs.forEach { (main, n) -> obs.subscribeOn(main) }
  }
}
`
	if got := runRuleByName(t, "SubscribeOnMain", kotlin); len(got) != 0 {
		t.Errorf("Kotlin: shadowed `main` must not report, got %v", got)
	}
	const java = `package app;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.core.Scheduler;
import java.util.List;

class J {
  private final Scheduler main = AndroidSchedulers.mainThread();

  void f(Observable<String> obs, List<Scheduler> list, Object o) {
    for (Scheduler main : list) obs.subscribeOn(main);
    try { } catch (RuntimeException main) { obs.subscribeOn(null); }
    if (o instanceof Scheduler main) obs.subscribeOn(main);
  }

  void g(Observable<String> obs) {
    obs.subscribeOn(main);
  }
}
`
	got := runRuleByNameOnJava(t, "SubscribeOnMain", java)
	if len(got) != 1 || got[0].Line != 18 {
		t.Errorf("Java: want only the field use on line 18, got %v", got)
	}
}

// Review of #1772 (P3): subscribeOn on RxJava 2 types in Java is not Lint's RxJava 3 owner.
func TestSubscribeOnMainJavaRxJava2Lookalike(t *testing.T) {
	const java = `package app;

import io.reactivex.Observable;
import io.reactivex.android.schedulers.AndroidSchedulers;

class J {
  void f(Observable<String> obs) {
    obs.subscribeOn(AndroidSchedulers.mainThread());
  }
}
`
	if got := runRuleByNameOnJava(t, "SubscribeOnMain", java); len(got) != 0 {
		t.Errorf("RxJava 2: want no finding, got %v", got)
	}
}

// Review of #1772 (P3): only a function that is in scope at the call shadows the imported factory, with Kotlin's
// priorities; and send only counts when ProducerScope is the receiver.
func TestRxDoesNotEmitScoping(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"unrelated member does not shadow", `import kotlinx.coroutines.rx3.rxObservable

class Other { fun rxObservable(x: Int) = x }

fun f() = rxObservable<String> { println("never") }
`, 1},
		{"member of the enclosing class shadows", `import kotlinx.coroutines.rx3.rxObservable

class Owner {
  fun rxObservable(block: () -> Unit) = block()
  fun f() = rxObservable { println("never") }
}
`, 0},
		{"explicit import beats a top-level function", `import kotlinx.coroutines.rx3.rxObservable

fun <T> rxObservable(x: Int) = x

fun f() = rxObservable<String> { println("never") }
`, 1},
		{"top-level function beats a star import", `import kotlinx.coroutines.rx3.*

fun rxObservable(block: () -> Unit) = block()

fun f() = rxObservable { println("never") }
`, 0},
		{"send on another receiver does not emit", `import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.rx3.rxObservable

fun f(ch: Channel<String>) = rxObservable<String> { ch.apply { send("x") } }
`, 1},
		{"with() rebinds the receiver", `import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.rx3.rxObservable

fun f(ch: Channel<String>) = rxObservable<String> { with(ch) { trySend("x") } }
`, 1},
		{"a local send shadows the member", `import kotlinx.coroutines.rx3.rxObservable

fun f() = rxObservable<String> {
  fun send(s: String) = println(s)
  send("x")
}
`, 1},
		{"this@ send still emits inside apply", `import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.rx3.rxObservable

fun f(ch: Channel<String>) = rxObservable<String> { ch.apply { this@rxObservable.send("x") } }
`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runRuleByName(t, "RxObservableDoesNotEmit", "package app\n\n"+tc.src); len(got) != tc.want {
				t.Errorf("want %d findings, got %v", tc.want, got)
			}
		})
	}
}
