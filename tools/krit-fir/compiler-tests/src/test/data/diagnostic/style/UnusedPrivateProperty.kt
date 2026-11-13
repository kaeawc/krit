// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 6, 8, 10, 13, 14, 15, 16, 17, 23, 24, 29, 34, 38, 43, 44, 45, 53, 68, 70, 72
// Positives: a private property or constructor parameter that nothing in its file refers to.
package test.unusedprivateproperty

private val <!UnusedPrivateProperty!>topLevel<!> = 1

private var <!UnusedPrivateProperty!>topLevelVar<!>: String = "x"

private const val <!UnusedPrivateProperty!>topLevelConst<!> = 3

class Members {
    private val <!UnusedPrivateProperty!>unusedVal<!> = 1
    private var <!UnusedPrivateProperty!>unusedVar<!>: Int = 2
    private lateinit var <!UnusedPrivateProperty!>unusedLate<!>: String
    private val <!UnusedPrivateProperty!>unusedDelegated<!> by lazy { 4 }
    private val <!UnusedPrivateProperty!>unusedWithGetter<!>: Int
        get() = 5

    fun used() = 1

    companion object {
        private const val <!UnusedPrivateProperty!>UNUSED_CONST<!> = 6
        private val <!UnusedPrivateProperty!>unusedCompanion<!> = "c"
    }
}

object Holder {
    private val <!UnusedPrivateProperty!>unusedInObject<!> = 7
}

class Nested {
    class Inner {
        private val <!UnusedPrivateProperty!>unusedInNested<!> = 8
    }

    inner class Deep {
        private val <!UnusedPrivateProperty!>unusedInInner<!> = 9
    }
}

class Constructors(
    private val <!UnusedPrivateProperty!>unusedCtorVal<!>: Int,
    private var <!UnusedPrivateProperty!>unusedCtorVar<!>: String,
    <!UnusedPrivateProperty!>plainParam<!>: Int,
    val publicParam: Int,
) {
    fun use() = publicParam
}

fun local() {
    val anonymous = object {
        private val <!UnusedPrivateProperty!>unusedInAnonymous<!> = 10
        fun go() = 1
    }
    println(anonymous.go())
}

// A property only written, never read, still counts as used, like Go.
class WriteOnly {
    private var written = 0
    fun poke() {
        written = 1
    }
}

// The parameter of an open and an abstract class is judged like any other.
open class OpenBase(<!UnusedPrivateProperty!>unusedOpen<!>: Int)

abstract class AbstractBase(private val <!UnusedPrivateProperty!>unusedAbstract<!>: Int)

enum class Level(private val <!UnusedPrivateProperty!>unusedEnumParam<!>: Int) {
    LOW(1),
    HIGH(2),
}
