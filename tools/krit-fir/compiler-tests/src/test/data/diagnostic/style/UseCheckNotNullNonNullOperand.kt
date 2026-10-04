// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 19, 25, 31, 37, 44, 51, 58, 65, 71, 76, 81, 86, 91
// Non-null operands Go cannot see: K2 types each compared value below as
// non-null, through a stable smart cast (a contract, `!!`, `as`, `when`, an
// equality with a non-null value, a stable property chain) or through its
// declared or inferred type (a `!!` or elvis initializer, a lambda or loop
// parameter of a non-null element type, a parenthesized non-null name). There
// is no nullable value to pass to checkNotNull, and `x != null` is always
// true. Go reports each of these: its resolver applies the same non-null
// skip, but only to a plain name narrowed by an early return or `if`, or
// declared non-null. Each function uses its own names so that Go's resolver
// does not carry a fact from one function into another.
package test

// kotlin.check's contract narrows `a`: Go reports both calls, FIR only the
// first.
fun checkTwice(a: String?) {
    <!UseCheckNotNull!>check(a != null)<!>
    check(a != null)
}

// kotlin.require's contract narrows `b`.
fun requireThenCheck(b: String?) {
    require(b != null)
    check(b != null)
}

// `c!!` narrows `c`.
fun notNullAssertion(c: String?) {
    c!!
    check(c != null)
}

// `d as String` narrows `d`.
fun castThenCheck(d: Any?) {
    d as String
    check(d != null)
}

// The `else` branch runs only when `e` is not null.
fun whenBranch(e: String?) {
    when {
        e == null -> return
        else -> check(e != null)
    }
}

// Equality with a non-null value narrows `f`.
fun equalityWithNonNull(f: String?, other: String) {
    if (f == other) {
        check(f != null)
    }
}

// A stable `val` property chain is narrowed like a local.
fun stablePropertyChain(g: Node) {
    if (g.next != null) {
        check(g.next != null)
    }
}

// A `!!` initializer makes the inferred type non-null.
fun notNullInitializer() {
    val h = System.getenv("KRIT")!!
    check(h != null)
}

// An elvis with a non-null fallback makes the inferred type non-null.
fun elvisInitializer(value: String?) {
    val i = value ?: "default"
    check(i != null)
}

// `it` in a safe-call `let` is the non-null receiver.
fun safeCallLet(j: String?) {
    j?.let { check(it != null) }
}

// A lambda parameter of a non-null element type.
fun lambdaParameter() {
    listOf<String>().forEach { k -> check(k != null) }
}

// A loop variable of a non-null element type.
fun loopVariable(items: List<String>) {
    for (l in items) check(l != null)
}

// A parenthesized name of a non-null declared type.
fun parenthesizedOperand(m: String) {
    check((m) != null)
}

class Node(val next: Node?)
