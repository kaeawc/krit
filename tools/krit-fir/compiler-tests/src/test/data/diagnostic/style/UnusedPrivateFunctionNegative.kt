// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: every function is called, or exempt.
package test.unusedprivatefunctionnegative

import kotlin.reflect.KProperty

annotation class Preview
annotation class Provides
annotation class Test
annotation class JvmStatic2
annotation class BigPreviews

private fun called() {}

private fun calledInLambda(): Int = 1

private fun referenced(): Int = 2

private fun inDefault(): Int = 3

class Calls {
    private fun member() = 1
    private fun viaThis() = 2
    private fun viaReference() = 3
    private fun boundReference() = 4
    private fun extra() = 5

    fun use(x: Int = extra()): Int {
        val lambda = { calledInLambda() }
        val callable = ::referenced
        val bound = this::boundReference
        val unbound = Calls::viaReference
        called()
        return member() + this.viaThis() + lambda() + callable() + bound() + unbound(this) + inDefault() + x
    }
}

class Substituted<T>(private val item: T) {
    private fun peek(): T = item
    fun read(other: Substituted<String>): String = other.peek()
}

object Registry {
    private fun register() {}
    fun add() = register()
}

class Companions {
    private fun fromCompanion() = 1

    companion object {
        private fun shared() = 2
        fun make(owner: Companions) = owner.fromCompanion() + shared()
    }
}

class Outer {
    private fun seenByNested() = 1

    class Nested(private val outer: Outer) {
        fun read() = outer.seenByNested()
    }

    inner class Deep {
        fun read() = seenByNested()
    }
}

private fun String.shout() = this + "!"

fun callShout() = "a".shout()

// Overloads are told apart by resolution, so each is judged on its own calls.
private fun over(a: Int) = a

private fun over(a: String) = a.length

fun callOver() = over(1) + over("x")

// Operators and conventions resolve to their functions.
class Vec(val x: Int) {
    private operator fun plus(other: Vec) = Vec(x + other.x)
    private operator fun get(index: Int) = x + index
    private operator fun invoke() = x
    private operator fun unaryMinus() = Vec(-x)
    private operator fun contains(value: Int) = value == x
    private operator fun component1() = x
    private operator fun iterator(): Iterator<Int> = listOf(x).iterator()
    private operator fun getValue(thisRef: Any?, property: KProperty<*>): Int = x
    private infix fun combine(other: Vec) = Vec(x + other.x)

    val delegated: Int by this

    fun use(other: Vec): Int {
        val (first) = other
        var total = (this + other)[0] + this() + (-this).x + first
        if (1 in this) total++
        for (item in this) total += item
        return total + (this combine other).x
    }
}

// Entry points the framework calls without a source reference.
@Preview
private fun previewing() {}

@BigPreviews
private fun wrappedPreview() {}

@Provides
private fun provided(): Int = 1

class Tests {
    @Test
    private fun testing() {}
}

@Suppress("unused")
@Test
private fun suppressedEntry() {}

// Not private: a public, internal, protected or local function is never judged.
fun public() {}

internal fun internalFun() {}

open class Open {
    protected fun protectedFun() {}

    fun localFunctions() {
        fun neverCalled() {}
    }
}

// A call from another private function that is itself called keeps both alive.
private fun outerHelper() = innerHelper()

private fun innerHelper() = 1

fun callOuterHelper() = outerHelper()

// A call in a property initializer, an init block and a nested function counts.
private fun forInit() = 1

private fun forProperty() = 2

private fun forNested() = 3

class Initializers {
    private val value = forProperty()

    init {
        forInit()
    }

    fun go(): Int {
        fun nested() = forNested()
        return value + nested()
    }
}
