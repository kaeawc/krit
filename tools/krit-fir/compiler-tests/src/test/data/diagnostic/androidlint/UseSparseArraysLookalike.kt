// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Divergence (precision): a same-package class named SparseArray, and an
// argument that is only named Int. Neither is the framework SparseArray of
// kotlin.Int, so FIR reports nothing. Go skips the first because the file has
// no android.util import, but reports the qualified call on line 16 because it
// reads the argument by its written name.
package test

class SparseArray<E>

class Int

val lookalikeArray = SparseArray<kotlin.Int>()

val lookalikeArgument = android.util.SparseArray<Int>()
