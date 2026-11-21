package kotlinx.atomicfu.locks

typealias ReentrantLock = java.util.concurrent.locks.ReentrantLock
fun reentrantLock(): ReentrantLock = ReentrantLock()
inline fun <T> synchronized(lock: Any, block: () -> T): T = kotlin.synchronized(lock, block)
inline fun <T> ReentrantLock.withLock(block: () -> T): T {
    lock()
    try { return block() } finally { unlock() }
}
