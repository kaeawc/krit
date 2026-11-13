// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): a typealias for the value type or for SparseArray. Each
// call constructs an android.util.SparseArray with a written Int or Boolean
// argument, so FIR reports it. Go reads the written names and misses them.
package test

import android.util.SparseArray as FrameworkArray

typealias Count = Int
typealias Flag = Boolean
typealias Sparse<V> = FrameworkArray<V>

val aliasedArgument = <!UseSparseArrays!>FrameworkArray<Count>()<!>

val aliasedFlag = <!UseSparseArrays!>FrameworkArray<Flag>()<!>

val aliasedClass = <!UseSparseArrays!>Sparse<Int>()<!>

// An alias whose argument is neither Int nor Boolean is reported by neither.
typealias Name = String

val aliasedName = FrameworkArray<Name>()
