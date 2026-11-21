package kotlinx.atomicfu

class AtomicInt(val value: Int) {
    operator fun getValue(thisRef: Any?, property: kotlin.reflect.KProperty<*>): Int = value
    operator fun setValue(thisRef: Any?, property: kotlin.reflect.KProperty<*>, value: Int) {}
}
class AtomicLong(val value: Long)
class AtomicBoolean(val value: Boolean)
class AtomicRef<T>(val value: T)
class AtomicIntArray(size: Int)
class AtomicLongArray(size: Int)
class AtomicBooleanArray(size: Int)
class AtomicRefArray<T>(size: Int, initialValue: T)

fun atomic(value: Int): AtomicInt = AtomicInt(value)
