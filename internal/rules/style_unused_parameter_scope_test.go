package rules_test

import (
	"strings"
	"testing"
)

func TestUnusedParameter_WhenSubjectInsideTrailingLambda(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Panel {
  private fun refresh(state: Mode) {
    if (root == null) {
      return
    }
    requireHost().runOnUiThread {
      when (state) {
        Mode.A -> view!!.isVisible = true
        Mode.B -> view!!.isVisible = false
      }
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestUnusedParameter_ArgumentWhenSiblingFunctionHasSameParameter(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Panel {
  private fun first(kind: Kind) {
    handleKind(kind)
  }

  private fun second(kind: Kind) {
    log(kind)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestUnusedParameter_WhenConditionInExpressionBody(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

internal fun resolve(
  override: Version?,
  isDebug: Boolean,
  isUnstable: Boolean,
): Destination =
  when {
    override == Version.V2 -> Destination.V2
    !isDebug && isUnstable -> Destination.OLD
    else -> Destination.NEW
  }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestUnusedParameter_AliasedFunctionTypeInvokedOnReceiver(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Runner {
  private suspend fun run(id: Id, name: String, action: HelperAction) {
    val helper = helperProvider.get()
    try {
      val meta = helper.resolve(id)
      helper.action(id, meta).getOrThrow()
    } catch (e: Exception) {
      log(name)
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestUnusedParameter_WhenSubjectInTrailingLambdaAfterMisparsedWith(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Panel {
  private fun refresh(state: Mode) {
    if (host == null) {
      return // Bail early.
    }
    with(binding.bar) {
      getIcon().visibility = View.GONE
    }
    if (item == null) {
      return
    }
    requireHost().runOnUiThread {
      when (state) {
        Mode.A -> item!!.isVisible = true
        Mode.B -> item!!.isVisible = false
      }
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestUnusedParameter_UsedInErrorNodeAfterEarlyEndedFunction(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

class Planner
@Inject
constructor(
  private val limit: Int,
) :
  Scheduler by clock {
  init {
    boot()
  }

  private fun reschedule(task: Optional<Task>, policy: Policy): List<String> {
    if (task.isPresent) {
      if (!queued) {
        log("rescheduling")
        if (enabled && hasCapacity) {
          plan = plan.copy(id = task.id)
          if (!reminders().addAll(dueSoon(), overdue, snoozed)) {
            refresh()
          }
          notifier.get().lastId = plan.lastId
          with(store.get()) {
            setId(task.id)
          }
          return task.subtaskIds
        }
      }
    } else if (queued) {
      dropOne(policy)
    }

    return emptyList()
  }

  private fun dropOne(policy: Policy) {
    plan = plan.copy(id = nextId())
    if (policy is Strict && archived) {
      halt()
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// A backticked parameter is read through the same backticked name; the reference must match it.
func TestUnusedParameter_BacktickedNameUsedWithBackticks(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", withBackticks(`
package test

class Cipher {
  fun encrypt(~in~: ByteArray) = native(~in~)
  fun decrypt(~is~: ByteArray): ByteArray {
    return native("${~is~}".toByteArray())
  }
  private fun native(bytes: ByteArray) = bytes
}
`))
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// An unused backticked parameter is still reported, with the name as written.
func TestUnusedParameter_BacktickedNameUnused(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", withBackticks(`
package test

fun encrypt(~in~: ByteArray) = 1
`))
	if want := withBackticks("Parameter '~in~' is unused."); len(findings) != 1 || findings[0].Message != want {
		t.Fatalf("expected one finding %q, got %v", want, findings)
	}
}

// withBackticks writes a Kotlin backtick for each ~, which a Go raw string cannot hold.
func withBackticks(s string) string { return strings.ReplaceAll(s, "~", "`") }
