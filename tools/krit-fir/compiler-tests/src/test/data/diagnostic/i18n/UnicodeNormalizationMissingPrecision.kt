// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 22, 24, 26, 28, 30, 32, 35, 38, 41, 47, 50, 54x2
// Precision: contains() over values that are not text. Go cannot see the
// receiver type, so it reports every call spelled contains() in a
// search/find function; here no characters are compared, so the message
// ("will miss unicode-equivalent characters") is false and FIR does not
// report them.
package unmprecision

import java.util.Locale
import java.util.UUID

data class Point(val x: Int, val y: Int)

enum class Status { ACTIVE, INACTIVE }

// Go reports these: the receivers are collections of numbers, ids, points,
// enums, locales, and a range, and every argument is of that element type.
// A data class made only of numbers compares no characters.
fun findIds(ids: List<Int>, id: Int): Boolean = ids.contains(id)

fun searchIds(ids: Set<UUID>, id: UUID): Boolean = ids.contains(id)

fun findPoint(points: List<Point>, point: Point): Boolean = points.contains(point)

fun searchStatus(allowed: Set<Status>, status: Status): Boolean = allowed.contains(status)

fun findLocale(supported: List<Locale>, locale: Locale): Boolean = supported.contains(locale)

fun findInRange(range: IntRange, value: Int): Boolean = range.contains(value)

fun searchKeys(byId: Map<Long, String>, id: Long): Boolean = byId.contains(id)

// A type parameter bounded by a non-text type.
fun <T : Number> findNumber(items: List<T>, value: T): Boolean = items.contains(value)

// A list of numbers compares only numbers.
fun findIdGroup(groups: Set<List<Int>>, ids: List<Int>): Boolean = groups.contains(ids)

// Arrays compare by identity, so a CharArray element compares no characters.
fun findChars(words: List<CharArray>, word: CharArray): Boolean = words.contains(word)

// A project class with a contains that takes a number.
class Bitset(private val bits: Long) {
    fun contains(bit: Int): Boolean = bits and (1L shl bit) != 0L

    fun findBit(bit: Int): Boolean = contains(bit)
}

fun searchBits(set: Bitset, bit: Int): Boolean = set.contains(bit)

// Mixed in one search function: the text call is still reported.
fun searchMixed(ids: List<Int>, id: Int, title: String, query: String): Boolean =
    ids.contains(id) && <!UnicodeNormalizationMissing!>title.contains(query)<!>
