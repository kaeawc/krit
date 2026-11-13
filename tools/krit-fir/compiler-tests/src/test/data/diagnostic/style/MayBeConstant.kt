// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 6, 7, 8, 9, 10, 11, 12, 13, 14, 17, 18, 19, 22, 23, 28, 34, 39
// Positive: top-level, object and companion `val`s whose initializer the compiler folds to a constant.
package test.maybeconstant

val <!MayBeConstant!>NAME<!> = "hello"
val <!MayBeConstant!>COUNT<!> = 42
val <!MayBeConstant!>ENABLED<!> = true
val <!MayBeConstant!>RATIO<!> = 1.5
val <!MayBeConstant!>BIG<!> = 10L
val <!MayBeConstant!>LETTER<!> = 'x'
val <!MayBeConstant!>NEGATIVE<!> = -1
val <!MayBeConstant!>ARITHMETIC<!> = 1 + 2 * 3
val <!MayBeConstant!>TYPED<!>: Long = 5L
// Go misses unsigned literals: `3u` is not in its literal list.
val <!MayBeConstant!>UNSIGNED<!>: UInt = 3u
private val <!MayBeConstant!>HIDDEN<!> = "hidden"
internal val <!MayBeConstant!>SHARED<!> = "shared"
val <!MayBeConstant!>`ticked name`<!> = 7

object Config {
    val <!MayBeConstant!>TAG<!> = "tag"
    private val <!MayBeConstant!>LIMIT<!> = 100
}

class Holder {
    companion object {
        val <!MayBeConstant!>KEY<!> = "key"
    }
}

interface Contract {
    companion object {
        val <!MayBeConstant!>VERSION<!> = 3
    }
}

data object Singleton {
    val <!MayBeConstant!>LABEL<!> = "label"
}
