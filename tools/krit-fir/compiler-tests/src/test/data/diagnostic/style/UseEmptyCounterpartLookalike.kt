// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 20, 26, 37, 38
// Lookalikes: project functions spelled like the stdlib factories. None of
// these calls is a stdlib factory, so the empty counterpart is not a
// replacement for it.
package test

// A same-package top-level `setOf()` shadows the default-imported one.
fun setOf(): Set<String> = hashSetOf("default")

// Divergence: Go reports any callee spelled `setOf`.
fun samePackage() {
    println(setOf())
}

class Defaults {
    fun listOf(): List<String> = arrayListOf("default")

    // Divergence: Go reports the member call spelled `listOf`.
    fun use(): List<String> = listOf()
}

// Divergence: Go reports the local function call spelled `mapOf`.
fun localShadow() {
    fun mapOf(): Map<String, Int> = hashMapOf("a" to 1)
    println(mapOf())
}

class Builder {
    fun arrayOf(): Array<String> = Array(1) { "default" }
    fun sequenceOf(): Sequence<String> = generateSequence { "default" }
}

// Divergence: Go reports the implicit-receiver member calls.
fun implicitReceiver(builder: Builder) {
    with(builder) {
        println(arrayOf())
        println(sequenceOf())
    }
}

// Neither reports an explicit-receiver member call.
fun explicitReceiver(builder: Builder) {
    println(builder.arrayOf())
    println(builder.sequenceOf())
}
