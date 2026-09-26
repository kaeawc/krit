// Smoke: suspending on a Play services Task with kotlinx.coroutines.tasks.await.
package stubs

import com.google.android.gms.tasks.Task
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.tasks.await

suspend fun awaitPlayServicesTasks(token: Task<String>, count: Task<Int>): String = coroutineScope {
    val pending = async { count.await() }
    token.await().repeat(pending.await())
}
