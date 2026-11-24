package typeinfer

import "testing"

func TestLambdaParametersShadowOuterNames(t *testing.T) {
	src := `
data class State(val items: List<String>?)

fun f(states: List<State>, all: List<List<String>?>): Any {
    val items: List<String> = emptyList()
    states.map { (items) ->
        val destructured = items
    }
    all.map { items ->
        val plain = items
    }
    return states.map { (items, _) ->
        val inReturn = items
    }
}
`
	for _, probe := range []string{"destructured", "plain", "inReturn"} {
		got := resolveProbeInitializer(t, src, probe)
		if got.Kind != TypeUnknown {
			t.Errorf("%s: lambda parameter `items` must shadow the outer List<String>, got %+v", probe, got)
		}
	}
}

func TestScopeFunctionLambdaThis(t *testing.T) {
	src := `
class Account(val plan: String)

class AccountManager {
    fun find(id: String): Account? = null
}

class Handler {
    fun onChange(manager: AccountManager, nullable: Account?, account: Account) {
        with(nullable) {
            val withNullable = this
        }
        nullable.run {
            val runNullable = this
        }
        nullable.apply {
            val applyNullable = this
        }
        nullable?.run {
            val safeRun = this
        }
        with(account) {
            val withAccount = this
        }
        with(manager.find("a")) {
            val withCall = this
        }
        buildString {
            val otherLambda = this
        }
    }
}
`
	for _, probe := range []string{"withNullable", "runNullable", "applyNullable"} {
		got := resolveProbeInitializer(t, src, probe)
		if !got.IsNullable() || got.Name != "Account" {
			t.Errorf("%s: want Account?, got %+v", probe, got)
		}
	}
	assertProvenNonNull(t, "safeRun", resolveProbeInitializer(t, src, "safeRun"), "Account")
	assertProvenNonNull(t, "withAccount", resolveProbeInitializer(t, src, "withAccount"), "Account")
	assertNotProvenNonNull(t, "withCall", resolveProbeInitializer(t, src, "withCall"))
	// Other receiver lambdas keep the enclosing `this`; rules guard those separately.
	assertProvenNonNull(t, "otherLambda", resolveProbeInitializer(t, src, "otherLambda"), "Handler")
}

func TestImplicitItKeepsElementNullability(t *testing.T) {
	src := `
fun f(names: List<String?>, ids: List<Int>) {
    names.forEach {
        val nullableIt = it
    }
    ids.forEach {
        val nonNullIt = it
    }
}
`
	if got := resolveProbeInitializer(t, src, "nullableIt"); !got.IsNullable() {
		t.Errorf("nullableIt: want String?, got %+v", got)
	}
	assertProvenNonNull(t, "nonNullIt", resolveProbeInitializer(t, src, "nonNullIt"), "Int")
}
