// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 37, 47, 53
// Smart casts: a value K2 has narrowed to non-null (a stable smart cast) is
// not reported, as Go skips a name its resolver has narrowed. An unstable
// smart cast (a `var` property) does not narrow, so it is reported.
package test

// Go narrows these as well and skips them.
fun earlyReturn(x: String?) {
    if (x == null) return
    check(x != null)
}

fun insideIf(x: String?) {
    if (x != null) {
        check(x != null)
    }
}

fun afterElvis(x: String?) {
    x ?: return
    check(x != null)
}

fun afterIsCheck(x: Any?) {
    if (x !is String) return
    check(x != null)
}

fun afterContract(x: String?) {
    requireNotNull(x)
    check(x != null)
}

fun unstableProperty(m: Mutable) {
    if (m.value == null) return
    <!UseCheckNotNull!>check(m.value != null)<!>
}

// A smart cast Go does not see: the local `var` x is non-null here, so there
// is no nullable value to pass to checkNotNull, and `x != null` is always
// true. Go reports it: its resolver does not narrow a local `var` after an
// early return.
fun stableLocalVar(y: String?) {
    var x = y
    if (x == null) return
    check(x != null)
}

// Not narrowed: y's check says nothing about x.
fun otherName(x: String?, y: String?) {
    if (y == null) return
    <!UseCheckNotNull!>check(x != null)<!>
}

class Mutable(var value: String?)
