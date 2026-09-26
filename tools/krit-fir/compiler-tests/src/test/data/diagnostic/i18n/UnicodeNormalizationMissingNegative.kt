// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: shapes neither Go nor FIR reports.
package unmnegative

import java.nio.file.Path
import java.text.Normalizer

data class User(val name: String)

// Both operands normalized.
fun searchUsers(users: List<User>, query: String): List<User> {
    val normalized = Normalizer.normalize(query, Normalizer.Form.NFC)
    return users.filter {
        Normalizer.normalize(it.name, Normalizer.Form.NFC)
            .contains(normalized, ignoreCase = true)
    }
}

// Any call written normalize anywhere in the function counts, even after
// the contains or inside a lambda or local function, whatever declares it,
// as in Go.
fun findLater(title: String, query: String): Boolean {
    val hit = title.contains(query)
    listOf(query).forEach { Normalizer.normalize(it, Normalizer.Form.NFD) }
    return hit
}

fun findLocalHelper(title: String, query: String): Boolean {
    fun normalize(s: String): String = s
    return normalize(title).contains(query)
}

fun findPath(path: Path, title: String, query: String): Boolean {
    path.normalize()
    return title.contains(query)
}

fun findNormalizerValue(title: String, query: String, normalize: (String) -> String): Boolean =
    normalize(title).contains(normalize(query))

fun findInDefault(title: String, query: String = Normalizer.normalize("q", Normalizer.Form.NFC)): Boolean =
    title.contains(query)

// Not a search / find function.
fun greet(name: String, suffix: String): Boolean = name.contains(suffix, ignoreCase = true)

fun lookup(title: String, query: String): Boolean = title.contains(query)

fun researchTitles(title: String, query: String): Boolean = title.contains(query)

// Backticks are part of the name as Go reads it, so it does not start with find.
fun `find by title`(title: String, query: String): Boolean = title.contains(query)

// The nearest named function decides: a local function nested in a search
// function is not a search function.
fun searchOuter(titles: List<String>, query: String): Int {
    fun matches(title: String): Boolean = title.contains(query)
    return titles.count { matches(it) }
}

// A member of an anonymous object inside a search function.
fun findOuter(query: String): Any = object {
    fun check(title: String): Boolean = title.contains(query)
}

// No enclosing function at all.
class Holder(title: String, query: String) {
    val hit: Boolean = title.contains(query)
    val lazyHit: Boolean
        get() = "abc".contains("b")
}

// The `in` operator and an infix call are not contains() call expressions.
infix fun String.contains(c: Int): Boolean = length > c

fun findOperator(title: String, query: String): Boolean = query in title

fun findNotIn(title: String, query: String): Boolean = query !in title

fun findInfix(title: String): Boolean = title contains 3

// A callable reference is not a call.
fun findReference(titles: List<String>, query: String): List<String> = titles.filter(query::contains)

// Other names.
fun findPrefix(title: String, query: String): Boolean = title.startsWith(query) || title.endsWith(query)
