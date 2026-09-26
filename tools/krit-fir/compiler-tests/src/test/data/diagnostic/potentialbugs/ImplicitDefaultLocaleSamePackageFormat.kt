// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// A same-package `String.Companion.format` shadows the stdlib one, so
// `String.format(...)` below calls this project function, which does no
// formatting. Go reports it (it does not resolve the call and only looks for a
// project `String.format` extension on string literal receivers); FIR does
// not. The same-package `String.format(vararg)` extension shadows the stdlib
// instance form too, which Go also skips.
package test

fun String.Companion.format(pattern: String, vararg args: Any?): String = pattern + args.size

fun String.format(vararg args: Any?): String = this + args.size

class ProjectFormat {
    fun static(value: Int): String = String.format("%d", value)

    fun instance(value: Int): String = "%d".format(value)
}
