package rules_test

import "testing"

func TestUnusedVariable_AnnotatedConstructorClassMembersAreNotLocals(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Header
@Inject
constructor(
  context: Context,
  val ext: Set<String>,
) : Base(context, null, 0) {
  private val binding = Binding.inflate(context, this)
  val icon: Icon = binding.icon
}
`)
	if len(findings) != 0 {
		t.Fatalf("class members must not be reported as locals, got %d", len(findings))
	}
}

func TestUnusedVariable_DelegatedClassBodyMembersAreNotLocals(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Holder(view: View) : Holders by KeyHolder() {
  private val binding = Binding.bind(view)
  val itemView: View = binding.itemView
}
`)
	if len(findings) != 0 {
		t.Fatalf("delegating class members must not be reported as locals, got %d", len(findings))
	}
}

func TestUnusedVariable_UsedByNextLineParenthesizedStatement(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun a(steps: List<Int>) {
  val entry =
    Data(
      title = "x",
    )
  (steps + entry).toList()
}
`)
	if len(findings) != 0 {
		t.Fatalf("use on a following parenthesized line must count, got %d", len(findings))
	}
}

func TestUnusedVariable_UsedByCallableReference(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun c(ids: List<Int>) {
  val idsSet = ids.toSet()
  ids.filter(idsSet::contains)
}
`)
	if len(findings) != 0 {
		t.Fatalf("callable reference receiver is a use, got %d", len(findings))
	}
}

func TestUnusedVariable_StillReportsUnusedLocalInFunction(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun c(ids: List<Int>) {
  val idsSet = ids.toSet()
  println(ids)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestVarCouldBeVal_AnnotatedConstructorClassMembersAreNotLocals(t *testing.T) {
	findings := runRuleByName(t, "VarCouldBeVal", `
package test

class Sink
@Inject
constructor(
  private val nav: Nav,
) : Base() {
  var callback: () -> Unit = {}
}

fun wire(s: Sink) {
  s.apply { callback = {} }
}
`)
	if len(findings) != 0 {
		t.Fatalf("members parsed as locals must not be reported, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_FunInterfaceBesideClassIsMultiDeclaration(t *testing.T) {
	findings := runRuleByNameOnPath(t, "MatchingDeclarationName", "Epsilon.kt", `
package test

class Zeta

fun interface Epsilon {
  fun run()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a file with two declarations, got %d", len(findings))
	}
}

func TestMatchingDeclarationName_SingleMismatchedClassStillReported(t *testing.T) {
	findings := runRuleByNameOnPath(t, "MatchingDeclarationName", "Epsilon.kt", `
package test

class Zeta
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestUnusedVariable_UsedInsideNextLineParenthesizedForEach(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun a(items: List<Int>, other: List<Int>) {
  val source = compute()
  (items + other).forEach { item ->
    println(item + source)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("use inside a lambda after a parenthesized line must count, got %d", len(findings))
	}
}

func TestUnusedVariable_UsedByAssignmentAfterNextLineParenthesizedStatement(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class A {
  fun bind(holder: Any, position: Int) {
    if (position > 0) {
      val adjusted = position - 1
      (holder as Row).view().tag = adjusted
      val item = items[adjusted]
      println(item)
    }
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("uses after a parse-folded assignment must count, got %d", len(findings))
	}
}

func TestUnusedVariable_UsedByNamedArgumentAfterElvisReturnAndNextLineParens(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class A {
  fun select(mode: Any) {
    val state =
      listeners.get().onSelect(mode)
        ?: return
    (mode as? Input)?.with(toolbar = state)?.let(::setMode)
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("use as a named argument after elvis-return must count, got %d", len(findings))
	}
}

func TestUnusedVariable_StillReportsUnusedBeforeNextLineParens(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun a(items: List<Int>) {
  val unusedValue = compute()
  (items + items).forEach { item ->
    println(item)
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the unused local to still be reported, got %d", len(findings))
	}
}

func TestUnusedVariable_WhenSubjectBindingSharesNameWithOuterLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

class Item(val text: Any?)

fun render(item: Item): String? {
    val label =
        when (val label = item.text) {
            is String -> label
            is Int -> label.toString()
            else -> ""
        }
    if (label.isEmpty()) return null
    return label
}
`)
	if len(findings) != 0 {
		t.Fatalf("outer local is used after the when; got %d findings: %v", len(findings), findings)
	}
}

func TestUnusedVariable_ForLoopVariableSharesNameWithOuterLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun render(items: List<String>): String {
    val label = "x"
    for (label in items) {
        println(label)
    }
    return label
}
`)
	if len(findings) != 0 {
		t.Fatalf("outer local is used after the loop; got %v", findings)
	}
}

func TestUnusedVariable_LambdaParameterSharesNameWithOuterLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun render(items: List<String>): String {
    val label = "x"
    items.forEach { label -> println(label) }
    return label
}
`)
	if len(findings) != 0 {
		t.Fatalf("outer local is used after the lambda; got %v", findings)
	}
}

func TestUnusedVariable_WhenSubjectBindingStillReportsUnusedOuter(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun render(item: Any?) {
    val label = "x"
    when (val label = item) {
        is String -> println(label)
        else -> Unit
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("outer local is never used; expected one finding, got %v", findings)
	}
}
