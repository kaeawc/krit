// Smoke: atomicfu scalar and array types.
package stubs

import kotlinx.atomicfu.AtomicArray
import kotlinx.atomicfu.AtomicBoolean
import kotlinx.atomicfu.AtomicBooleanArray
import kotlinx.atomicfu.AtomicInt
import kotlinx.atomicfu.AtomicIntArray
import kotlinx.atomicfu.AtomicLong
import kotlinx.atomicfu.AtomicLongArray
import kotlinx.atomicfu.AtomicRef
import kotlinx.atomicfu.atomic

fun atomicfuSmoke(): Int {
    val int = AtomicInt(1)
    val long = AtomicLong(2L)
    val boolean = AtomicBoolean(true)
    val ref = AtomicRef("value")
    val intArray = AtomicIntArray(3)
    val longArray = AtomicLongArray(4)
    val booleanArray = AtomicBooleanArray(5)
    val array = AtomicArray<String>(6)
    val delegated by atomic(7)
    return int.value + long.value.toInt() + boolean.value.compareTo(false) + intArray[0].value +
        longArray[0].value.toInt() + booleanArray[0].value.compareTo(false) + ref.value.length +
        array[0].value.length + intArray.size + longArray.size + booleanArray.size + array.size + delegated
}
