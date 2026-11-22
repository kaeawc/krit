package kotlinx.atomicfu

import kotlin.reflect.KProperty

class AtomicInt(initial: Int = 0) {
    var value: Int = initial
    operator fun getValue(thisRef: Any?, property: KProperty<*>): Int = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: Int) {
        this.value = value
    }
}

class AtomicLong(initial: Long = 0L) {
    var value: Long = initial
    operator fun getValue(thisRef: Any?, property: KProperty<*>): Long = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: Long) {
        this.value = value
    }
}

class AtomicBoolean(initial: Boolean = false) {
    var value: Boolean = initial
    operator fun getValue(thisRef: Any?, property: KProperty<*>): Boolean = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: Boolean) {
        this.value = value
    }
}

class AtomicRef<T>(initial: T) {
    var value: T = initial
    operator fun getValue(thisRef: Any?, property: KProperty<*>): T = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: T) {
        this.value = value
    }
}

class AtomicIntArray(val size: Int) {
    operator fun get(index: Int): AtomicInt = AtomicInt()
}

class AtomicLongArray(val size: Int) {
    operator fun get(index: Int): AtomicLong = AtomicLong()
}

class AtomicBooleanArray(val size: Int) {
    operator fun get(index: Int): AtomicBoolean = AtomicBoolean()
}

class AtomicArray<T>(val size: Int) {
    operator fun get(index: Int): AtomicRef<T> = AtomicRef(TODO())
}

fun atomic(initial: Int): AtomicInt = AtomicInt(initial)
fun atomic(initial: Long): AtomicLong = AtomicLong(initial)
fun atomic(initial: Boolean): AtomicBoolean = AtomicBoolean(initial)
fun <T> atomic(initial: T): AtomicRef<T> = AtomicRef(initial)
