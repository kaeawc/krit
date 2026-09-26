// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 14, 17, 19, 22, 24, 27, 31, 36x2, 39, 41, 44, 46, 49, 51, 54, 60, 63, 67, 70, 75, 81, 87, 89, 96, 100
// Positive: contains() over text inside a function whose name starts with
// search / find (any case) that never calls normalize, in every container Go
// visits. Go and FIR both report on the line where the call expression starts.
package unm

data class User(val name: String)

fun searchUsers(users: List<User>, query: String): List<User> =
    users.filter { <!UnicodeNormalizationMissing!>it.name.contains(query, ignoreCase = true)<!> }

fun findUser(users: List<User>, query: String): User? =
    users.firstOrNull { <!UnicodeNormalizationMissing!>it.name.contains(query)<!> }

// Case-insensitive prefix match on the function name.
fun SearchTitles(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>

fun FINDALL(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>

// Char and Regex overloads of CharSequence.contains.
fun findAccent(title: String): Boolean = <!UnicodeNormalizationMissing!>title.contains('é')<!>

fun searchPattern(title: String, pattern: Regex): Boolean = <!UnicodeNormalizationMissing!>title.contains(pattern)<!>

// Safe call on a nullable receiver.
fun findNullable(title: String?, query: String): Boolean = <!UnicodeNormalizationMissing!>title?.contains(query)<!> == true

// A chained receiver: reported on the line where the chain starts.
fun searchChained(title: String, query: String): Boolean =
    <!UnicodeNormalizationMissing!>title<!>
        .trim()
        .contains(query)

// Two calls on one line: one finding each.
fun findBoth(a: String, b: String, query: String): Boolean = <!UnicodeNormalizationMissing!>a.contains(query)<!> || <!UnicodeNormalizationMissing!>b.contains(query)<!>

// A collection of strings compares the strings.
fun searchTags(tags: List<String>, query: String): Boolean = <!UnicodeNormalizationMissing!>tags.contains(query)<!>

fun findInSet(tags: Set<String>, query: String): Boolean = <!UnicodeNormalizationMissing!>tags.contains(query)<!>

// Values that may hold text: Any, and an unbounded type parameter.
fun findAny(items: List<Any>, query: Any): Boolean = <!UnicodeNormalizationMissing!>items.contains(query)<!>

fun <T> findGeneric(items: List<T>, query: T): Boolean = <!UnicodeNormalizationMissing!>items.contains(query)<!>

// Implicit receivers: an extension body and a scope-function lambda.
fun String.findIn(query: String): Boolean = <!UnicodeNormalizationMissing!>contains(query)<!>

fun searchWith(title: String, query: String): Boolean = with(title) { <!UnicodeNormalizationMissing!>contains(query)<!> }

// A StringBuilder is a CharSequence.
fun searchBuffer(buffer: StringBuilder, query: String): Boolean = <!UnicodeNormalizationMissing!>buffer.contains(query)<!>

// A project function named contains that takes text: Go reports it by name,
// and the operands are text.
fun contains(haystack: String, needle: String): Boolean = haystack.indexOf(needle) >= 0

fun searchProject(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>contains(title, query)<!>

// A function-typed value named contains, invoked with text.
fun findWithMatcher(query: String, contains: (String) -> Boolean): Boolean = <!UnicodeNormalizationMissing!>contains(query)<!>

// Members, companions, objects, and local classes.
class Repository(private val titles: List<String>) {
    fun searchTitles(query: String): List<String> = titles.filter { <!UnicodeNormalizationMissing!>it.contains(query)<!> }

    companion object {
        fun findFirst(titles: List<String>, query: String): String? = titles.firstOrNull { <!UnicodeNormalizationMissing!>it.contains(query)<!> }
    }
}

object Finder {
    fun findTitle(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>
}

interface Searchable {
    val title: String

    fun searchSelf(query: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>
}

// The nearest named function decides: lambdas, anonymous functions, and
// local classes inside a search function are looked through.
fun searchNested(titles: List<String>, query: String): Int {
    val matcher = fun(title: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>
    class Local(val title: String) {
        val matches: Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>
    }
    return titles.count { matcher(it) && Local(it).matches }
}

// A member of an anonymous object named like a search function.
fun other(query: String): Any = object {
    fun findHere(title: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>
}

// A contains in a default value of a search function's parameter.
fun findDefault(title: String, query: String, hit: Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>): Boolean = hit
