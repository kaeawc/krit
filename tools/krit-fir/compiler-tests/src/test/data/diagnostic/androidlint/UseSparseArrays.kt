// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 13, 14, 15, 16, 17, 18, 19, 22, 26, 30
// Positives: an android.util.SparseArray constructor call with a written Int,
// Integer, or Boolean type argument, however SparseArray is spelled and
// whatever the constructor arguments. A nullable argument counts as its box,
// as in Android Lint 32.4.1. Go reports each of these too.
package test

import android.util.SparseArray

class Cache {
    val ints = <!UseSparseArrays!>SparseArray<Int>()<!>
    val booleans = <!UseSparseArrays!>SparseArray<Boolean>()<!>
    val boxedInts = <!UseSparseArrays!>SparseArray<Integer>()<!>
    val nullableInts = <!UseSparseArrays!>SparseArray<Int?>()<!>
    val nullableBooleans = <!UseSparseArrays!>SparseArray<Boolean?>()<!>
    val withCapacity = <!UseSparseArrays!>SparseArray<Int>(10)<!>
    val qualified = <!UseSparseArrays!>android.util.SparseArray<Boolean>()<!>
    val qualifiedArgument = <!UseSparseArrays!>SparseArray<kotlin.Int>()<!>
}

fun chained() = <!UseSparseArrays!>SparseArray<Int>()<!>.apply { put(1, 2) }

fun multiline() {
    val array =
        <!UseSparseArrays!>SparseArray<Boolean>()<!>
    array.put(1, true)
}

fun inLambda(): () -> SparseArray<Int> = { <!UseSparseArrays!>SparseArray<Int>()<!> }
