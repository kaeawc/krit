// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 24, 30, 41, 42, 67, 68, 77, 84
// Lookalikes: project declarations and import aliases spelled like the stdlib
// factories. None of these calls is a stdlib factory, so the empty
// counterpart is not a replacement for it.
package test

import kotlin.emptyArray as arrayOf
import test.Factories.listOf
import test.Factories.sequenceOf

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

// An explicitly imported project function and an imported object with
// `operator fun invoke()`, both spelled like a factory. The explicit imports
// shadow the default-imported stdlib factories. UseEmptyCounterpartCrossFileTest
// covers the same shapes imported from another package.
object Factories {
    fun listOf(): List<String> = arrayListOf("default")

    object sequenceOf {
        operator fun invoke(): Sequence<Int> = generateSequence { 1 }
    }
}

// Divergence: Go reports the imported function call spelled `listOf` and the
// imported object's invoke call spelled `sequenceOf`.
fun imported() {
    println(listOf())
    println(sequenceOf())
}

// A same-package property of function type: `listOfNotNull()` is an implicit
// invoke of the property, which shadows the default-imported function.
val listOfNotNull: () -> List<Int> = { emptyList() }

// Divergence: Go reports the invoke call spelled `listOfNotNull`.
fun propertyInvoke() {
    println(listOfNotNull())
}

// Divergence: `arrayOf` is imported as an alias of `emptyArray`, so the call is
// already the empty counterpart. Go reports it by its written name.
// UseEmptyCounterpartAnnotationAlias.kt covers the annotation form.
fun aliasedCounterpart() {
    val a: Array<String> = arrayOf()
    println(a)
}
