// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 18, 26, 70, 76, 80, 98, 102, 106
package scopeCancellation

import kotlinx.coroutines.*
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineScope as OwnedScope
import kotlinx.coroutines.flow.*
import kotlin.coroutines.EmptyCoroutineContext

class OrdinaryOwner {
    private val <!CoroutineScopeCreatedButNeverCancelled!>scope<!> = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun start() { scope.launch {} }
}

class LocalOwner {
    fun start() {
        val <!CoroutineScopeCreatedButNeverCancelled!>local<!> = CoroutineScope(Dispatchers.Default)
        local.launch {}
    }
}

class MultilineOwner {
    @Suppress("unused")
    private
    val <!CoroutineScopeCreatedButNeverCancelled!>multiline<!> = CoroutineScope(EmptyCoroutineContext)
}

class AliasedOwner {
    // Go misses the aliased factory; FIR resolves kotlinx.coroutines.CoroutineScope.
    val <!CoroutineScopeCreatedButNeverCancelled!>aliased<!> = OwnedScope(Dispatchers.IO)
}

class MainOwner {
    // Go only matches CoroutineScope, but MainScope also creates an independently owned scope.
    val <!CoroutineScopeCreatedButNeverCancelled!>main<!> = MainScope()
}

class ShadowedCancellation {
    // Go sees a same-named receiver cancelled elsewhere; FIR distinguishes declarations.
    val <!CoroutineScopeCreatedButNeverCancelled!>shadowed<!> = CoroutineScope(Dispatchers.IO)
    fun stop(shadowed: CoroutineScope) { shadowed.cancel() }
}

class ProjectCancellation {
    // Go counts any cancel name; this resolved project extension does not cancel anything.
    val <!CoroutineScopeCreatedButNeverCancelled!>project<!> = CoroutineScope(Dispatchers.IO)
    fun stop() { project.cancel(1) }
    private fun CoroutineScope.cancel(unused: Int) { println(unused) }
}

class MisleadingParentName {
    // Go exempts the parentJob token, but the resolved dispatcher supplies no parent Job.
    val <!CoroutineScopeCreatedButNeverCancelled!>parentJobScope<!> = CoroutineScope(Dispatchers.IO)
}

class NullParent {
    // Go treats Job(null) as parent-owned; resolution proves there is no parent.
    val <!CoroutineScopeCreatedButNeverCancelled!>unparented<!> = CoroutineScope(Job(null) + Dispatchers.IO)
}

class ProjectChildrenCancellation {
    // Match Go conservatively: even a project cancelChildren extension counts as management.
    val uncancelled = CoroutineScope(Dispatchers.IO)
    fun close() { uncancelled.cancelChildren(1) }
    private fun CoroutineScope.cancelChildren(unused: Int) {}
}

class BorrowingOwner(source: Flow<Int>) {
    val <!CoroutineScopeCreatedButNeverCancelled!>borrowed<!> = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    val state = source.stateIn(borrowed, SharingStarted.Eagerly, 0)
}

class NestedContainers {
    companion object {
        val <!CoroutineScopeCreatedButNeverCancelled!>companion<!> = CoroutineScope(Dispatchers.IO)
    }
    fun create() {
        class Nested {
            val <!CoroutineScopeCreatedButNeverCancelled!>nested<!> = CoroutineScope(Dispatchers.IO)
        }
    }
}

object Standalone {
    val scope = CoroutineScope(Dispatchers.IO)
}
val topLevel = CoroutineScope(Dispatchers.IO)

annotation class Singleton
@Singleton
class LookalikeAnnotation {
    // Match Go's simple-name ownership exemption for resolved project annotation classes.
    val notSingleton = CoroutineScope(Dispatchers.IO)
}

class HandlerContext {
    val <!CoroutineScopeCreatedButNeverCancelled!>scope<!> = CoroutineScope(SupervisorJob() + Dispatchers.IO + CoroutineExceptionHandler { _, _ -> })
}

class NamedContext {
    val <!CoroutineScopeCreatedButNeverCancelled!>scope<!> = CoroutineScope(Dispatchers.IO + CoroutineName("sync"))
}

class UnrelatedLambdaCancellation {
    val <!CoroutineScopeCreatedButNeverCancelled!>scope<!> = CoroutineScope(Dispatchers.IO)
    fun stop(other: CoroutineScope) { other.let { it.cancel() } }
}
