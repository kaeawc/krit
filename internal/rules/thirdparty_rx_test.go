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
