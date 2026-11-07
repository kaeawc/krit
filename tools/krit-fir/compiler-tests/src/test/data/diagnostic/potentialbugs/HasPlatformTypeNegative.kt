// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: a declared type, a non-public declaration, a non-platform
// inferred type, and local declarations.
package test

import java.io.File

class Negatives {
    fun declared(): String = File("a").name

    fun declaredNullable(): String? = File("a").name

    private fun privateFun() = File("a").name

    internal fun internalFun() = File("a").name

    private val privateProperty = System.getenv("HOME")

    internal val internalProperty = System.getenv("HOME")

    val declaredProperty: String = System.getenv("HOME")

    // Elvis, `!!` and a call result of a Kotlin type are not flexible.
    fun elvis() = File("a").name ?: ""

    fun bang() = File("a").name!!

    fun length() = File("a").name.length

    fun primitive() = File("a").isDirectory

    fun kotlinOnly() = listOf(1, 2).first()

    fun unitBlock() {
        File("a").name
    }

    fun local() {
        fun inner() = File("a").name
        val value = File("a").name
        inner()
        value.length
    }

    val delegated by lazy { 1 }

    val destructured: Int
        get() {
            val (a, b) = 1 to 2
            return a + b
        }
}

fun localClassMember() {
    class Local {
        fun member() = File("a").name
    }
    Local().member()
}

private fun topLevelPrivate() = File("a").name

private val topLevelPrivateProperty = System.getenv("HOME")

fun main() {
    topLevelPrivate()
    println(topLevelPrivateProperty)
}
