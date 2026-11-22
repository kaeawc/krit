package stubs

import kotlinx.atomicfu.*

class AtomicSmoke {
    var delegatedInt by atomic(1)
    var delegatedLong by atomic(2L)
    var delegatedBoolean by atomic(true)
    var delegatedRef by atomic("initial")

    fun exercise(): Int {
        val int = AtomicInt(3)
        val long = AtomicLong(4L)
        val boolean = AtomicBoolean(false)
        val ref = AtomicRef("value")
        val ints = AtomicIntArray(1)
        val longs = AtomicLongArray(1)
        val booleans = AtomicBooleanArray(1)
        val refs = AtomicArray<String>(1)
        delegatedInt++
        delegatedLong++
        delegatedBoolean = true
        delegatedRef = "updated"
        return int.value + long.value.toInt() + boolean.value.compareTo(false) + ref.value.length +
            ints[0].value + longs[0].value.toInt() + booleans[0].value.compareTo(false) +
            refs[0].value.length + ints.size + longs.size + booleans.size + refs.size +
            atomic(5).value + atomic(6L).value.toInt() + atomic(false).value.compareTo(false) +
            atomic("atomic").value.length
    }
}
