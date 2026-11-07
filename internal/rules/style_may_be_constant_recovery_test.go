package rules_test

import "testing"

func TestMayBeConstant_PrivateConstructorHeaderIsNotAProperty(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

class Invoice
private constructor(
  owner: Owner,
  currency: String = "EUR",
  onSent: (() -> Unit)? = null,
) : Document(owner, currency, onSent) {

  sealed class Layout(open val columns: Int, open val rows: Int? = null) {
    data object Compact : Layout(columns = 2)

    data class Grid(override val columns: Int, override val rows: Int) : Layout(columns, rows)
  }

  class Builder {
    private var layout: Layout = Layout.Compact

    fun layout(value: Layout) = apply { this.layout = value }

    fun build(owner: Owner): Document = Invoice(owner, "EUR", null)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("a recovered constructor header is not a property, got %d", len(findings))
	}
}

func TestMayBeConstant_TopLevelStatementsAfterRecoveryErrorAreNotProperties(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

class Tests {
  @Test
  fun first() {
    run()
  }
}
}

  val filename = "filename"
  run(filename)
`)
	if len(findings) != 0 {
		t.Fatalf("locals stranded after a parse error are not properties, got %d", len(findings))
	}
}

func TestMayBeConstant_StillReportsTopLevelAndCompanionVals(t *testing.T) {
	findings := runRuleByName(t, "MayBeConstant", `
package test

val TOP = "top"

class Holder {
  companion object {
    val LIMIT = 10
  }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both constant vals to be reported, got %d", len(findings))
	}
}

// detekt reports MayBeConstant at the property's name identifier, after any annotations and modifiers.
func TestMayBeConstant_AnchorsAtTheName(t *testing.T) {
	code := `package test
internal val PLAIN = "a"
@VisibleForTesting internal val SAME_LINE = "b"
@VisibleForTesting
internal val OWN_LINE = "c"
class Holder {
    @VisibleForTesting val MEMBER_OF_OBJECT_LIKE = 5000L
}
object Constants {
    @VisibleForTesting val IN_OBJECT: Long = -1L
}
`
	findings := runRuleByName(t, "MayBeConstant", code)
	got := map[magicHit]string{}
	for _, f := range findings {
		got[magicHit{f.Line, f.Col}] = f.Message
	}
	for _, name := range []string{"PLAIN", "SAME_LINE", "OWN_LINE", "IN_OBJECT"} {
		want := lineCol(t, code, name)
		msg, ok := got[want]
		if !ok {
			t.Errorf("expected a finding at %v for %s, got %v", want, name, got)
			continue
		}
		if msg != name+" can be a `const val`." {
			t.Errorf("message for %s = %q", name, msg)
		}
	}
	if len(got) != 4 {
		t.Errorf("expected 4 findings, got %v", got)
	}
}
