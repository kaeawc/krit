package dev.jasonpearson.krit.fir.tests

import org.junit.jupiter.api.Test
import kotlin.test.fail

// IteratorNotThrowingNoSuchElementException cases a single-file golden cannot
// express: an Iterator or NoSuchElementException lookalike declared in another
// file of the same package, or imported (explicitly or with a star import) from
// another package, which shadows the stdlib name without the using file
// declaring it. The Go rule only sees one file, so it cannot tell these
// lookalikes from the stdlib types.
class IteratorNotThrowingNoSuchElementExceptionTest {

    private data class Case(val file: String, val source: String, val expected: Int, val why: String)

    private val support = mapOf(
        "SamePackageIterator.kt" to """
            package samepackage

            interface Iterator<T> {
                fun hasNext(): Boolean
                fun next(): T
            }
        """.trimIndent(),
        "SamePackageException.kt" to """
            package samepackageexception

            class NoSuchElementException(message: String? = null) : RuntimeException(message)
        """.trimIndent(),
        "OtherPackageException.kt" to """
            package b.errors

            class NoSuchElementException(message: String? = null) : RuntimeException(message)
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

                class Cursor(private val items: List<Int>) : Iterator<Int> {
                    private var index = 0

                    override fun hasNext(): Boolean = index < items.size

                    override fun next(): Int = items[index++]
                }
            """.trimIndent(),
            expected = 0,
            why = "a same-package Iterator lookalike has no NoSuchElementException contract " +
                "(Go reports it: it cannot see the declaration in the other file)",
        ),
        Case(
            "ImportsOtherIterator.kt",
            """
                package importsother

                import com.example.cursor.Iterator

                class Cursor(private val items: List<Int>) : Iterator<Int> {
                    private var index = 0

                    override fun hasNext(): Boolean = index < items.size

                    override fun next(): Int = items[index++]
                }
            """.trimIndent(),
            expected = 0,
            why = "an imported Iterator lookalike is not an iterator (Go resolves the import too)",
        ),
        Case(
            "UsesSamePackageException.kt",
            """
                package samepackageexception

                class Cursor(private val items: List<Int>) : Iterator<Int> {
                    private var index = 0

                    override fun hasNext(): Boolean = index < items.size

                    override fun next(): Int {
                        if (!hasNext()) throw NoSuchElementException("exhausted")
                        return items[index++]
                    }
                }
            """.trimIndent(),
            expected = 1,
            why = "a same-package NoSuchElementException lookalike is not java.util.NoSuchElementException " +
                "(Go misses it: it cannot see the declaration in the other file)",
        ),
        Case(
            "ImportsOtherException.kt",
            """
                package importsotherexception

                import b.errors.NoSuchElementException

                class Cursor(private val items: List<Int>) : Iterator<Int> {
                    private var index = 0

                    override fun hasNext(): Boolean = index < items.size

                    override fun next(): Int {
                        if (!hasNext()) throw NoSuchElementException("x")
                        return items[index++]
                    }
                }
            """.trimIndent(),
            expected = 1,
            why = "an imported NoSuchElementException lookalike is not java.util.NoSuchElementException " +
                "(Go misses it: it matches the call by its simple name)",
        ),
        Case(
            "StarImportsOtherException.kt",
            """
                package starimportsotherexception

                import b.errors.*

                class Cursor(private val items: List<Int>) : Iterator<Int> {
                    private var index = 0

                    override fun hasNext(): Boolean = index < items.size

                    override fun next(): Int {
                        if (!hasNext()) throw NoSuchElementException("x")
                        return items[index++]
                    }
                }
            """.trimIndent(),
            expected = 1,
            why = "a star-imported NoSuchElementException lookalike wins over the default kotlin import and is " +
                "not java.util.NoSuchElementException (Go misses it: it matches the call by its simple name)",
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
        const val RULE = "IteratorNotThrowingNoSuchElementException"
    }
}
