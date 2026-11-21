package kotlinx.atomicfu.locks

typealias SynchronizedObject = Any
typealias ReentrantLock = java.util.concurrent.locks.ReentrantLock

fun reentrantLock(): ReentrantLock = TODO()
inline fun <T> synchronized(lock: SynchronizedObject, block: () -> T): T = TODO()
inline fun <T> ReentrantLock.withLock(block: () -> T): T = TODO()
