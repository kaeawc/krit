// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 25, 27, 31, 35, 38
// Positives: a java.util.HashMap constructor call with an Int, Integer, or Long
// key, however HashMap is spelled and whatever the constructor arguments. Go
// reports each of these too.
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

fun <T> generic(): MutableMap<Int, T> = <!UseSparseArrays!>HashMap<Int, T>()<!>

fun chained(): Map<Int, String> = <!UseSparseArrays!>HashMap<Int, String>()<!>.apply { put(1, "a") }

fun multiline(): Map<Int, String> {
    val map =
        <!UseSparseArrays!>HashMap<Int, String>()<!>
    return map
}

fun inLambda(): () -> Map<Int, String> = { <!UseSparseArrays!>HashMap<Int, String>()<!> }

val anonymous = object {
    fun make(): Map<Long, Int> = <!UseSparseArrays!>HashMap<Long, Int>()<!>
}
