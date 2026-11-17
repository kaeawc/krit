// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergences from Go, which keeps a class whenever its name shows up elsewhere in the file.
package test.unusedprivateclassdivergence

// Go keeps this: another declaration has a member of the same name.
private class <!UnusedPrivateClass!>Clash<!>

object Declares {
    val Clash = 1
}

fun useClash() = Declares.Clash

// Go keeps this: a local variable has the same name.
private class <!UnusedPrivateClass!>Shadow<!>

fun useShadow(): Int {
    val Shadow = 3
    return Shadow
}

// The go-lines run (the Go rule alone, in memory) reports neither of these; the full Go pipeline reports both.
private fun interface <!UnusedPrivateClass!>Callback<!> {
    fun call()
}

// The finding stays on the name when annotations span several lines.
annotation class Unrelated

@Unrelated
@Deprecated("old")
private class <!UnusedPrivateClass!>MultiLine<!>
