// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 11, 12, 13, 14, 15, 16, 17, 22, 32
// Explicit this as the single receiver reference, and what else counts. Go has no types, so it reports only the explicit-this forms listed in the go-lines header; every other marker is a FIR-only finding.
package test

class Box { var size = 0; fun grow() {}; val items = mutableListOf<Int>() }
fun topLevel() {}

fun reported(box: Box, amount: Int, other: Box) {
    box.<!UnnecessaryApply!>apply<!> { this.grow() } // F01
    box.<!UnnecessaryApply!>apply<!> { this.size = amount } // F02
    box.<!UnnecessaryApply!>apply<!> { this.items.add(amount) } // F03
    box.<!UnnecessaryApply!>apply<!> { this.size += 1 } // F04
    box.<!UnnecessaryApply!>apply<!> { this.items[0] = 1 } // F05
    box.<!UnnecessaryApply!>apply<!> { this.also { it.grow() } } // F06
    box.<!UnnecessaryApply!>apply<!> { (this).grow() } // F07
    box.<!UnnecessaryApply!>apply<!> { this.size = size } // F08
    box.apply { this.grow(); this.grow() } // F09
    box.apply { this.size + 1 } // F10
    box.apply { this?.grow() } // F11
    box.<!UnnecessaryApply!>apply<!> { run { this.grow() } } // F12
    box.<!UnnecessaryApply!>apply<!> { this.add2(listOf(amount).size) } // F13
}
fun Box.add2(x: Int) {}
fun Box.ext() {
    apply { this.grow() } // F14
}

// Explicit this passed to another object is still the single receiver reference, as in detekt, so it is reported.
class Stack { fun removeAll(items: List<Int>) {} }
fun passedAsArgument(stack: Stack, ids: List<Int>) {
    ids.<!UnnecessaryApply!>apply<!> { stack.removeAll(this) } // F15
}
