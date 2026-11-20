package stubs

import kotlinx.coroutines.sync.*

suspend fun mutexSmoke() {
    val mutex = Mutex()
    if (!mutex.withLock { mutex.isLocked }) return
    mutex.lock()
    mutex.unlock()
    if (mutex.tryLock()) mutex.unlock()
    mutex.holdsLock(Any())
}
