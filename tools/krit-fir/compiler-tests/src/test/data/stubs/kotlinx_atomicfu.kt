package kotlinx.atomicfu

import kotlin.reflect.KProperty

class AtomicInt internal constructor() {
    var value: Int = 0
    operator fun getValue(thisRef: Any?, property: KProperty<*>): Int = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: Int) {
        this.value = value
    }
}

class AtomicLong internal constructor() {
    var value: Long = 0L
    operator fun getValue(thisRef: Any?, property: KProperty<*>): Long = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: Long) {
        this.value = value
    }
}

class AtomicBoolean internal constructor() {
    var value: Boolean = false
    operator fun getValue(thisRef: Any?, property: KProperty<*>): Boolean = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: Boolean) {
        this.value = value
    }
}

class AtomicRef<T> internal constructor() {
    var value: T = TODO()
    operator fun getValue(thisRef: Any?, property: KProperty<*>): T = value
    operator fun setValue(thisRef: Any?, property: KProperty<*>, value: T) {
        this.value = value
    }
}

fun atomic(initial: Int): AtomicInt = TODO()
fun atomic(initial: Long): AtomicLong = TODO()
fun atomic(initial: Boolean): AtomicBoolean = TODO()
fun <T> atomic(initial: T): AtomicRef<T> = TODO()
