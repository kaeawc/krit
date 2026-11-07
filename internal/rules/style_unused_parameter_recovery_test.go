package rules_test

import "testing"

func TestUnusedParameter_UsedInBodyStrandedByErrorRecovery(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

fun record(sensorId: String, reading: Reading) {
  counterFor(sensorId).latest = reading
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnusedParameter_AssignedThroughSafeCallChain(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

fun AlertDialog.update(title: CharSequence?, message: CharSequence?) {
  findViewById<TextView>(R.id.title)?.text = title
  findViewById<TextView>(R.id.text)?.text = message
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnusedParameter_InvokedInObjectLiteralGetterWithSameNamedMember(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

fun explosive(isDebug: () -> Boolean = { throw NotImplementedError() }) =
  Fake(
    object : Config {
      override val isDebug: Boolean
        get() = isDebug()
    }
  )
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnusedParameter_StillReportsUnusedWithErrorRecovery(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

fun record(sensorId: String, reading: Reading) {
  counterFor().latest = reading
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestUnusedParameter_UsedInBodySplitAcrossSiblingsByErrorRecovery(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Panel {
  private fun render(
    caption: String,
    style: Style,
    @DrawableRes badge: Int,
  ) {
    visibility = View.VISIBLE
    binding.thumb?.visibility = View.GONE
    badgeView().visibility = View.VISIBLE
    badgeView().setImageResource(badge)
    painter
      .get()
      .draw(
        fallback = caption,
        options =
          Options.builder()
            .style(style)
            .build(),
      )
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("uses stranded in sibling nodes must count, got %d", len(findings))
	}
}

func TestUnusedParameter_StillReportsUnusedWhenBodySplitByErrorRecovery(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Panel {
  private fun render(first: String, second: Style) {
    visibility = View.VISIBLE
    binding.thumb?.visibility = View.GONE
    badgeView().visibility = View.VISIBLE
  }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both unused parameters to be reported, got %d", len(findings))
	}
}
