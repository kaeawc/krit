// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 21
// Shapes where Go and the checker disagree.
package test.unusedparameterdivergence

class Screen {
    fun onClick() {}
}

// Go misses this: `screen.onClick()` is a call of the written name, which Go counts as an invocation of
// the function-type parameter. It calls a member of Screen; the parameter itself is never invoked.
fun memberCallSharesName(<!UnusedParameter!>onClick<!>: () -> Unit, screen: Screen) {
    screen.onClick()
}

// Go reports this: it reads a stub only when `= TODO(` sits on one line, so a line break after `=` hides it.
// The parameter of a placeholder body is not expected to be read, so the checker leaves it alone.
fun brokenStubExpression(value: Int): Int =
    TODO("later")

fun brokenUnitExpression(value: Int) =
    Unit
