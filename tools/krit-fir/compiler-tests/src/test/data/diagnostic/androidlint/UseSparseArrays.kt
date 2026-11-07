// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 26, 28, 32, 36, 39
// Positives: a java.util.HashMap constructor call with an Int, Integer, or Long
// key, however HashMap is spelled and whatever the constructor arguments. Go
// reports each of these too. A map that escapes as a Map is not reported; see
// UseSparseArraysEscapes.kt.
package test

class Cache {
    val intKeys = <!UseSparseArrays!>HashMap<Int, String>()<!>
    val integerKeys = <!UseSparseArrays!>HashMap<Integer, String>()<!>
    val longKeys = <!UseSparseArrays!>HashMap<Long, String>()<!>
    val booleanValues = <!UseSparseArrays!>HashMap<Int, Boolean>()<!>
    val intValues = <!UseSparseArrays!>HashMap<Integer, Integer>()<!>
    val longValues = <!UseSparseArrays!>HashMap<Int, Long>()<!>
    val nullableValues = <!UseSparseArrays!>HashMap<Int, String?>()<!>
    val listValues = <!UseSparseArrays!>HashMap<Int, List<String>>()<!>
    val withCapacity = <!UseSparseArrays!>HashMap<Int, String>(16)<!>
    val copied = <!UseSparseArrays!>HashMap<Int, String>(mapOf(1 to "a"))<!>
    val jdkQualified = <!UseSparseArrays!>java.util.HashMap<Int, String>()<!>
    val kotlinQualified = <!UseSparseArrays!>kotlin.collections.HashMap<Long, String>()<!>
    val qualifiedKey = <!UseSparseArrays!>HashMap<kotlin.Int, String>()<!>
    val boxedLongKey = <!UseSparseArrays!>HashMap<java.lang.Long, String>()<!>
}

fun <T> generic(): HashMap<Int, T> = <!UseSparseArrays!>HashMap<Int, T>()<!>

fun chained(): HashMap<Int, String> = <!UseSparseArrays!>HashMap<Int, String>()<!>.apply { put(1, "a") }

fun multiline() {
    val map =
        <!UseSparseArrays!>HashMap<Int, String>()<!>
    map[1] = "a"
}

fun inLambda(): () -> HashMap<Int, String> = { <!UseSparseArrays!>HashMap<Int, String>()<!> }

val anonymous = object {
    fun make(): HashMap<Long, Int> = <!UseSparseArrays!>HashMap<Long, Int>()<!>
}
