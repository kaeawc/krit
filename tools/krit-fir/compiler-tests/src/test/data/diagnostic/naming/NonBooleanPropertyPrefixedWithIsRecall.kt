// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// True positives Go misses: each property below is named `is` + uppercase
// letter and is not a Boolean, so Go's message is true of it, but Go does not
// report it.
package test

class Recall(
    // Go misses these: Go visits property declarations, not primary-constructor
    // `val`/`var` parameters, which are properties too.
    val <!NonBooleanPropertyPrefixedWithIs!>isId<!>: String,
    private var <!NonBooleanPropertyPrefixedWithIs!>isCount<!>: Int,
    // A plain constructor parameter is not a property.
    isPlain: String,
) {
    // Go misses these: without a declared type Go never looks at the type.
    // The inferred types are String, Int, List<String>.
    val <!NonBooleanPropertyPrefixedWithIs!>isName<!> = "hello"

    val <!NonBooleanPropertyPrefixedWithIs!>isCounted<!> = isCount + 1

    val <!NonBooleanPropertyPrefixedWithIs!>isList<!> = listOf(isPlain)

    val <!NonBooleanPropertyPrefixedWithIs!>isLazy<!> by lazy { isId.length }

    val <!NonBooleanPropertyPrefixedWithIs!>isGetter<!> get() = isId

    // The text ": " in the initializer is not a declared type either.
    val <!NonBooleanPropertyPrefixedWithIs!>isLabel<!> = "key: value"

    fun local(): Int {
        // Go misses this local for the same reason: the inferred type is Int.
        val <!NonBooleanPropertyPrefixedWithIs!>isSize<!> = isList.size
        return isSize
    }

    // Go misses this: the backticks are part of Go's identifier text, so the
    // name does not start with "is"; the property is named isQuoted.
    val <!NonBooleanPropertyPrefixedWithIs!>`isQuoted`<!>: String = "quoted"

    // Go misses these: an inferred Nothing? / Nothing is not Boolean. (K2
    // rejects an inferred Nothing on a member, not on a local.)
    fun pending(flag: kotlin.Boolean): String {
        val <!NonBooleanPropertyPrefixedWithIs!>isMissing<!> = null
        if (isMissing != null) return "never"
        if (flag) {
            val <!NonBooleanPropertyPrefixedWithIs!>isFailed<!> = error("bad: state")
        }
        if (!flag) {
            val <!NonBooleanPropertyPrefixedWithIs!>isThrown<!> = throw IllegalStateException("a: b")
        }
        val <!NonBooleanPropertyPrefixedWithIs!>isPending<!> = TODO()
        return isPending
    }

    // Go misses this: the inferred type is an anonymous object.
    private val <!NonBooleanPropertyPrefixedWithIs!>isAnon<!> = object { val x = 1 }

    // Go misses this: a `when` subject variable is a property, but not a
    // property declaration in Go's tree.
    fun subject(): String = when (val <!NonBooleanPropertyPrefixedWithIs!>isSubject<!> = isList.size) {
        0 -> "none"
        else -> isSubject.toString()
    }

    class Shadow {
        class Boolean

        // Go misses this: the declared type text is `Boolean`, but it names
        // the nested class Shadow.Boolean, not kotlin.Boolean.
        val <!NonBooleanPropertyPrefixedWithIs!>isShadowed<!>: Boolean = Boolean()
    }
}

// Go misses these: value and annotation class constructor properties are
// class parameters, not property declarations, in Go's tree.
@JvmInline
value class ValueRecall(val <!NonBooleanPropertyPrefixedWithIs!>isValue<!>: Int)

annotation class AnnotationRecall(val <!NonBooleanPropertyPrefixedWithIs!>isLabel<!>: String)
