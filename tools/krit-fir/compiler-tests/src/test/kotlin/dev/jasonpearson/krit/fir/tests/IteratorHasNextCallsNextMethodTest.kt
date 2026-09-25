package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.fail

// IteratorHasNextCallsNextMethod cases a single-file golden cannot express: an
// Iterator lookalike declared in another file of the same package, or imported
// from another package, which shadows the stdlib name without the using file
// declaring it. The Go rule only sees one file, so it cannot tell a
// same-package lookalike from the stdlib type.
class IteratorHasNextCallsNextMethodTest {

    private data class Case(val file: String, val source: String, val expected: Int, val why: String)

    private val support = mapOf(
        "SamePackageIterator.kt" to """
            package samepackage

            interface Iterator<T> {
                fun hasNext(): Boolean
                fun next(): T
            }
        """.trimIndent(),
        "OtherPackageIterator.kt" to """
            package com.example.cursor

            interface Iterator<T> {
                fun hasNext(): Boolean
                fun next(): T
            }
        """.trimIndent(),
    )

    private val cases = listOf(
        Case(
            "UsesSamePackageIterator.kt",
            """
                package samepackage

                class Cursor(private val items: Iterator<Int>) : Iterator<Int> {
                    override fun hasNext(): Boolean = items.next() > 0

                    override fun next(): Int = items.next()
                }
            """.trimIndent(),
            expected = 0,
            why = "a same-package Iterator lookalike is not an iterator " +
                "(Go reports it: it cannot see the declaration in the other file)",
        ),
        Case(
            "ImportsOtherIterator.kt",
            """
                package importsother

                import com.example.cursor.Iterator

                class Cursor(private val items: Iterator<Int>) : Iterator<Int> {
                    override fun hasNext(): Boolean = items.next() > 0

                    override fun next(): Int = items.next()
                }
            """.trimIndent(),
            expected = 0,
            why = "an imported Iterator lookalike is not an iterator (Go resolves the import too)",
        ),
        Case(
            "SamePackageRealIterator.kt",
            """
                package samepackage

                class RealCursor(private val items: kotlin.collections.Iterator<Int>) :
                    kotlin.collections.Iterator<Int> {
                    override fun hasNext(): Boolean = items.next() > 0

                    override fun next(): Int = items.next()
                }
            """.trimIndent(),
            expected = 1,
            why = "a qualified kotlin.collections.Iterator is the real iterator even next to a lookalike",
        ),
    )

    @Test
    fun lookalikesDeclaredInOtherFiles() {
        val diags = KritFirProbe.diagnose(support + cases.associate { it.file to it.source })
        val failures = cases.mapNotNull { case ->
            val n = diags.count { it.file == case.file && it.name == RULE }
            if (n == case.expected) null else "[${case.file}] expected ${case.expected}, got $n: ${case.why}"
        }
        val stray = diags.filter { it.name == RULE && it.file in support.keys }
        if (stray.isNotEmpty()) fail("unexpected findings in support files: $stray")
        if (failures.isNotEmpty()) fail(failures.joinToString("\n"))
    }

    private companion object {
        const val RULE = "IteratorHasNextCallsNextMethod"
    }
}
