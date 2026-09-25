// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: the stdlib uppercase() / lowercase() called without a Locale
// argument on an explicit receiver, in every container Go visits.
package ulim

fun topLevel(userName: String, email: String): String {
    val upper = <!UpperLowerInvariantMisuse!>userName.uppercase()<!>
    val lower = <!UpperLowerInvariantMisuse!>email.lowercase()<!>
    return upper + lower
}

// Char has its own kotlin.text.uppercase() / lowercase() returning String.
fun chars(initial: Char): String = <!UpperLowerInvariantMisuse!>initial.uppercase()<!> + <!UpperLowerInvariantMisuse!>initial.lowercase()<!>

// Safe call on a nullable receiver.
fun nullable(name: String?): String? = <!UpperLowerInvariantMisuse!>name?.lowercase()<!>

// A chained receiver: Go and FIR both report on the line where the chain starts.
fun chained(title: String): String =
    <!UpperLowerInvariantMisuse!>title<!>
        .trim()
        .uppercase()

fun chainedSafe(title: String?): String? =
    <!UpperLowerInvariantMisuse!>title<!>
        ?.trim()
        ?.lowercase()

// Nested: the outer call's receiver is itself a flagged call, one finding each
// (both on this line; the golden compares lines, the probe test counts them).
fun nested(label: String): String = <!UpperLowerInvariantMisuse!>label.uppercase().lowercase()<!>

// Inside a lambda and a string template.
fun inLambda(words: List<String>): List<String> = words.map { <!UpperLowerInvariantMisuse!>it.lowercase()<!> }

fun inTemplate(nickname: String): String = "Hello ${<!UpperLowerInvariantMisuse!>nickname.uppercase()<!>}"

fun capitalize(word: String): String = word.replaceFirstChar { <!UpperLowerInvariantMisuse!>it.uppercase()<!> }

// `this` receiver in an extension.
fun String.shout(): String = <!UpperLowerInvariantMisuse!>this.uppercase()<!>

class Member(private val display: String) {
    fun normalized(): String = <!UpperLowerInvariantMisuse!>display.lowercase()<!>

    val prop: String get() = <!UpperLowerInvariantMisuse!>display.uppercase()<!>

    companion object {
        fun fromCompanion(value: String): String = <!UpperLowerInvariantMisuse!>value.uppercase()<!>
    }
}

object Holder {
    fun inObject(value: String): String = <!UpperLowerInvariantMisuse!>value.lowercase()<!>
}

interface Normalizer {
    fun normalize(value: String): String = <!UpperLowerInvariantMisuse!>value.lowercase()<!>
}

fun anonymous(): Any = object {
    fun inAnonymous(value: String): String = <!UpperLowerInvariantMisuse!>value.uppercase()<!>
}

fun local(value: String): String {
    class Local {
        fun inLocal(): String = <!UpperLowerInvariantMisuse!>value.lowercase()<!>
    }
    fun localFun(): String = <!UpperLowerInvariantMisuse!>value.uppercase()<!>
    return Local().inLocal() + localFun()
}

// A parenthesized receiver.
fun parenthesized(first: String, last: String): String = <!UpperLowerInvariantMisuse!>(first + last).uppercase()<!>

// Initializers, default values, enum entry and supertype constructor arguments.
val topLevelProperty: String = <!UpperLowerInvariantMisuse!>"Title".lowercase()<!>

fun defaultValue(name: String, key: String = <!UpperLowerInvariantMisuse!>name.lowercase()<!>): String = key

enum class Level(val label: String) {
    Info(<!UpperLowerInvariantMisuse!>"info".uppercase()<!>),
}

open class Base(val tag: String)

class Derived(name: String) : Base(<!UpperLowerInvariantMisuse!>name.uppercase()<!>) {
    val folded: String

    init {
        folded = <!UpperLowerInvariantMisuse!>tag.lowercase()<!>
    }
}
