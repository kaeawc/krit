package kotlinx.atomicfu.locks

typealias SynchronizedObject = Any
typealias ReentrantLock = java.util.concurrent.locks.ReentrantLock

fun reentrantLock(): ReentrantLock = ReentrantLock()

inline fun <T> synchronized(lock: SynchronizedObject, block: () -> T): T =
    kotlin.synchronized(lock, block)

inline fun <T> ReentrantLock.withLock(block: () -> T): T {
    lock()
    try {
        return block()
    } finally {
        unlock()
    }
}
