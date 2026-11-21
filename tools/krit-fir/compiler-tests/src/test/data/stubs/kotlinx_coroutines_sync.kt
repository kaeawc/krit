package kotlinx.coroutines.sync

interface Mutex {
    val isLocked: Boolean
    suspend fun lock(owner: Any? = null)
    fun unlock(owner: Any? = null)
    fun tryLock(owner: Any? = null): Boolean
    fun holdsLock(owner: Any): Boolean
}

fun Mutex(locked: Boolean = false): Mutex = TODO()
suspend inline fun <T> Mutex.withLock(owner: Any? = null, action: () -> T): T = TODO()
