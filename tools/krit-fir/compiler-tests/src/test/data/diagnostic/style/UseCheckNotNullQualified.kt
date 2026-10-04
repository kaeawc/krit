// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 31, 35, 39, 44, 51
// Qualified nullable properties: a companion or object property of type
// String?, and a mutable property reached through a parameter, with the
// declaring classes placed before the functions. Each value is nullable at
// the call (a mutable property keeps its nullable type after an early
// return), so every call below should be checkNotNull. Go reports each of
// them on this file as well.
package test

class Settings {
    companion object {
        var current: String? = null
        val initial: String? = System.getenv("KRIT")
    }
}

class OneLine { companion object { var value: String? = null } }

object Registry {
    var entry: String? = null
}

class MutableFirst(var value: String?)

fun companionVar() {
    <!UseCheckNotNull!>check(Settings.current != null)<!>
}

fun companionVal() {
    <!UseCheckNotNull!>check(Settings.initial != null)<!>
}

fun explicitCompanion() {
    <!UseCheckNotNull!>check(Settings.Companion.current != null)<!>
}

fun oneLineCompanion() {
    <!UseCheckNotNull!>check(OneLine.value != null)<!>
}

fun objectVar() {
    if (Registry.entry == null) return
    <!UseCheckNotNull!>check(Registry.entry != null)<!>
}

// The same shape as unstableProperty in UseCheckNotNullSmartCast.kt, with the
// class declared before the function instead of after it.
fun unstableAfterClass(m: MutableFirst) {
    if (m.value == null) return
    <!UseCheckNotNull!>check(m.value != null)<!>
}
