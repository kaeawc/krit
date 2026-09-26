// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 21, 23, 25, 27, 29, 31, 34, 40, 43, 47x2
// Precision: contains() over values that are not text. Go cannot see the
// receiver type, so it reports every call spelled contains() in a
// search/find function; here no characters are compared, so the message
// ("will miss unicode-equivalent characters") is false and FIR does not
// report them.
package unmprecision

import java.util.Locale
import java.util.UUID

data class User(val id: Long, val name: String)

enum class Status { ACTIVE, INACTIVE }

// Go reports these: the receivers are collections of numbers, ids, users,
// enums, locales, and a range, and every argument is of that element type.
fun findIds(ids: List<Int>, id: Int): Boolean = ids.contains(id)

fun searchIds(ids: Set<UUID>, id: UUID): Boolean = ids.contains(id)

fun findUser(users: List<User>, user: User): Boolean = users.contains(user)

fun searchStatus(allowed: Set<Status>, status: Status): Boolean = allowed.contains(status)

fun findLocale(supported: List<Locale>, locale: Locale): Boolean = supported.contains(locale)

fun findInRange(range: IntRange, value: Int): Boolean = range.contains(value)

fun searchKeys(byId: Map<Long, String>, id: Long): Boolean = byId.contains(id)

// A type parameter bounded by a non-text type.
fun <T : Number> findNumber(items: List<T>, value: T): Boolean = items.contains(value)

// A project class with a contains that takes a number.
class Bitset(private val bits: Long) {
    fun contains(bit: Int): Boolean = bits and (1L shl bit) != 0L

    fun findBit(bit: Int): Boolean = contains(bit)
}

fun searchBits(set: Bitset, bit: Int): Boolean = set.contains(bit)

// Mixed in one search function: the text call is still reported.
fun searchMixed(ids: List<Int>, id: Int, title: String, query: String): Boolean =
    ids.contains(id) && <!UnicodeNormalizationMissing!>title.contains(query)<!>
