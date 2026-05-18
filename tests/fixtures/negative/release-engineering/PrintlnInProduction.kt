package com.example.releaseengineering

fun main() {
    println("script entry point")
}

// Bare `println(...)` resolves to a same-file top-level declaration
// rather than kotlin.io.println, so the rule must not flag it. Member-
// and local-function shadowing cases are exercised by direct unit tests
// rather than fixture files: the fixture harness ties one rule per
// .kt file, so we can only express one shadowing flavour here.
fun println(label: String): String = label

fun usesShadowed() {
    val recorded = println("ignored: shadowed by the top-level fun above")
    require(recorded.isNotEmpty())
}
