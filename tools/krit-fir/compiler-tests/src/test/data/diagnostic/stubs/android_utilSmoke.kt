// Smoke: every Log overload shape, SparseArray indexing, Size/SizeF.
package stubs

import android.util.AttributeSet
import android.util.Log
import android.util.Size
import android.util.SizeF
import android.util.SparseArray

private const val TAG = "Smoke"

fun logAll(error: Throwable, attrs: AttributeSet?, nullableTag: String?) {
    Log.v(TAG, "verbose")
    Log.d(TAG, "debug")
    Log.d(nullableTag, "debug with nullable tag", error)
    Log.i(TAG, "info")
    Log.i(TAG, "info", error)
    Log.w(TAG, "warn")
    Log.w(TAG, "warn", error)
    Log.w(TAG, error)
    Log.e(TAG, "error")
    Log.e(TAG, "error", error)
    Log.wtf(TAG, "wtf")
    Log.wtf(TAG, "wtf", error)
    if (Log.isLoggable(TAG, Log.DEBUG)) Log.d(TAG, Log.getStackTraceString(error))
    val sparse = SparseArray<String>()
    sparse.put(1, "one")
    val one: String? = sparse[1]
    val fallback: String = sparse.get(2, "two")
    val area = Size(1, 2).width * SizeF(1f, 2f).height
    println("$one $fallback $area ${sparse.size()} ${attrs?.attributeCount}")
}
