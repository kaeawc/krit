package kotlinx.atomicfu

class AtomicInt(var value: Int) {
    operator fun getValue(thisRef: Any?, property: kotlin.reflect.KProperty<*>): Int = value
    operator fun setValue(thisRef: Any?, property: kotlin.reflect.KProperty<*>, value: Int) {}
}

class AtomicLong(var value: Long)

class AtomicBoolean(var value: Boolean)

class AtomicRef<T>(var value: T)

class AtomicIntArray(val size: Int) {
    operator fun get(index: Int): AtomicInt = AtomicInt(0)
}

class AtomicLongArray(val size: Int) {
    operator fun get(index: Int): AtomicLong = AtomicLong(0L)
}

class AtomicBooleanArray(val size: Int) {
    operator fun get(index: Int): AtomicBoolean = AtomicBoolean(false)
}

class AtomicArray<T>(val size: Int) {
    operator fun get(index: Int): AtomicRef<T> = TODO()
}

fun atomic(value: Int): AtomicInt = AtomicInt(value)
