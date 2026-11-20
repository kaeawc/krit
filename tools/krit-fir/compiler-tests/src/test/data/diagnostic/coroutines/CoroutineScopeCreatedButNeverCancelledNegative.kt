// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 40, 82, 95, 102, 108
package scopeCancellationNegative

import kotlinx.coroutines.*
import kotlinx.coroutines.cancel
import kotlinx.coroutines.cancel as stopScope
import kotlin.coroutines.CoroutineContext
import javax.inject.Singleton
import com.squareup.anvil.annotations.ContributesBinding

class Cancelled {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() { scope.cancel() }
}

class ContextCancelled {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() { scope.coroutineContext.cancel() }
}

class JobCancelled {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() { scope.coroutineContext[Job]?.cancel() }
}

class ChildrenCancelled {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() { scope.coroutineContext.job.cancelChildren() }
}

class JoinedCancellation {
    // Go does not recognize cancelAndJoin; FIR resolves the Job extension that cancels before joining.
    val joined = CoroutineScope(Dispatchers.IO)
    suspend fun close() { joined.coroutineContext[Job]?.cancelAndJoin() }
}

class AliasedCancellation {
    // Go misses the aliased cancel call; FIR resolves the library extension.
    val aliased = CoroutineScope(Dispatchers.IO)
    fun close() { aliased.stopScope() }
}

class HelperCancelled {
    val scope = CoroutineScope(Dispatchers.IO)
    fun close() { stop() }
    private fun stop() { scope.cancel() }
}

class ParentOwned(parentJob: Job) {
    val scope = CoroutineScope(Job(parentJob) + Dispatchers.IO)
}

class StructuredOwner(coroutineContext: CoroutineContext) {
    val scope = CoroutineScope(coroutineContext)
}

class FakeOwner {
    private val scope = CoroutineScope(Dispatchers.IO)
}

@Singleton
class SingletonOwner {
    private val scope = CoroutineScope(Dispatchers.IO)
    fun close() {}
}

class SessionScope
@ContributesBinding(SessionScope::class)
class GraphOwner {
    private val scope = CoroutineScope(Dispatchers.IO)
}

class HandedOver {
    val scope = CoroutineScope(Dispatchers.IO)
    val consumer = Consumer(scope)
}
class Consumer(scope: CoroutineScope)

class ProjectBorrowingName {
    // Go assumes every stateIn borrows; this resolved project function passes ownership to a consumer.
    val transferred = CoroutineScope(Dispatchers.IO)
    val consumer = stateIn(transferred)
    private fun stateIn(scope: CoroutineScope): Consumer = Consumer(scope)
}

class Collected {
    val scope = CoroutineScope(Dispatchers.IO)
    val scopes = listOf(scope)
    fun close() { scopes.forEach { it.cancel() } }
}

class AliasCancelled {
    // Go misses cancellation through a differently named declaration; FIR follows the immutable alias.
    val original = CoroutineScope(Dispatchers.IO)
    private val alias = original
    fun close() { alias.cancel() }
}

class ReturnedOwner {
    // Go does not recognize returns as ownership transfer; FIR conservatively gives the caller ownership.
    val returned = CoroutineScope(Dispatchers.IO)
    fun giveAway(): CoroutineScope = returned
}

class ProjectFactory {
    // Go matches the simple factory name; FIR resolves this project declaration, not the library factory.
    val lookalike = CoroutineScope(1)
    private fun CoroutineScope(unused: Int): String = "not a scope"
}

class NullableCancellation {
    val nullable: CoroutineScope? = CoroutineScope(Dispatchers.IO)
    fun close() { nullable?.cancel() }
}

class MainCancelled {
    val main = MainScope()
    fun close() { main.cancel() }
}
