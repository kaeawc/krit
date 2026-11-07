// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 9, 14x2, 17, 21, 26, 32x2, 35, 37, 39, 42, 45, 47, 50, 55, 59, 63, 68, 70, 75, 80, 82, 85, 90, 94
// Positive: the stdlib uppercase() / lowercase() called without a Locale
// argument on an explicit receiver, in every container Go visits.
package ulim

fun topLevel(userName: String, email: String): String {
    val upper = Text(<!UpperLowerInvariantMisuse!>userName.uppercase()<!>)
    val lower = Text(<!UpperLowerInvariantMisuse!>email.lowercase()<!>)
    return upper + lower
}

// Char has its own kotlin.text.uppercase() / lowercase() returning String.
fun chars(initial: Char): String = Text(<!UpperLowerInvariantMisuse!>initial.uppercase()<!>) + Text(<!UpperLowerInvariantMisuse!>initial.lowercase()<!>)

// Safe call on a nullable receiver.
fun nullable(name: String?): String? = Text(<!UpperLowerInvariantMisuse!>name?.lowercase()<!>)

// A chained receiver: Go and FIR both report on the line where the chain starts.
fun chained(title: String): String =
    Text(<!UpperLowerInvariantMisuse!>title<!>
        .trim()
        .uppercase())

fun chainedSafe(title: String?): String? =
    Text(<!UpperLowerInvariantMisuse!>title<!>
        ?.trim()
        ?.lowercase())

// Nested: the outer call's receiver is itself a flagged call, one finding each
// (both on this line; the golden compares lines, the probe test counts them).
fun nested(label: String): String = Text(<!UpperLowerInvariantMisuse!>label.uppercase().lowercase()<!>)

// Inside a lambda and a string template.
fun inLambda(words: List<String>): List<String> = words.map { Text(<!UpperLowerInvariantMisuse!>it.lowercase()<!>) }

fun inTemplate(nickname: String): String = "Hello ${Text(<!UpperLowerInvariantMisuse!>nickname.uppercase()<!>)}"

fun capitalize(word: String): String = word.replaceFirstChar { Text(<!UpperLowerInvariantMisuse!>it.uppercase()<!>) }

// `this` receiver in an extension.
fun String.shout(): String = Text(<!UpperLowerInvariantMisuse!>this.uppercase()<!>)

class Member(private val display: String) {
    fun normalized(): String = Text(<!UpperLowerInvariantMisuse!>display.lowercase()<!>)

    val prop: String get() = Text(<!UpperLowerInvariantMisuse!>display.uppercase()<!>)

    companion object {
        fun fromCompanion(value: String): String = Text(<!UpperLowerInvariantMisuse!>value.uppercase()<!>)
    }
}

object Holder {
    fun inObject(value: String): String = Text(<!UpperLowerInvariantMisuse!>value.lowercase()<!>)
}

interface Normalizer {
    fun normalize(value: String): String = Text(<!UpperLowerInvariantMisuse!>value.lowercase()<!>)
}

fun anonymous(): Any = object {
    fun inAnonymous(value: String): String = Text(<!UpperLowerInvariantMisuse!>value.uppercase()<!>)
}

fun local(value: String): String {
    class Local {
        fun inLocal(): String = Text(<!UpperLowerInvariantMisuse!>value.lowercase()<!>)
    }
    fun localFun(): String = Text(<!UpperLowerInvariantMisuse!>value.uppercase()<!>)
    return Local().inLocal() + localFun()
}

// A parenthesized receiver.
fun parenthesized(first: String, last: String): String = Text(<!UpperLowerInvariantMisuse!>(first + last).uppercase()<!>)

// Initializers, default values, enum entry and supertype constructor arguments.
val titleText: String = "Title"

val topLevelProperty: String = Text(<!UpperLowerInvariantMisuse!>titleText.lowercase()<!>)

fun defaultValue(name: String, key: String = Text(<!UpperLowerInvariantMisuse!>name.lowercase()<!>)): String = key

enum class Level(val label: String) {
    Info(Text(<!UpperLowerInvariantMisuse!>titleText.uppercase()<!>)),
}

open class Base(val tag: String)

class Derived(name: String) : Base(Text(<!UpperLowerInvariantMisuse!>name.uppercase()<!>)) {
    val folded: String

    init {
        folded = Text(<!UpperLowerInvariantMisuse!>tag.lowercase()<!>)
    }
}

fun Text(text: String?): String = text.orEmpty()
