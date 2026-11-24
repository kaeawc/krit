// fir-parity: skip Intentional unresolved external types and missing members; Go remains authoritative on this error file.
package fixtures.negative.potentialbugs

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.first

class UselessElvisOnNonNull {
    // Nullable parameter — elvis is meaningful.
    fun nullableParam(s: String?) {
        val y = s ?: "fallback"
    }

    // Mutable var — Kotlin cannot smart-cast; the source resolver should not
    // claim non-null even when initialized with a non-null literal.
    var name: String = "hello"

    fun mutableVar() {
        val y = name ?: "fallback"
    }

    // Safe call chain — left side is nullable.
    fun safeCallChain(s: String?) {
        val y = s?.length ?: 0
    }

    // Dotted member access on unresolved receiver — bare-name resolver
    // cannot prove non-null.
    fun dottedAccess(harness: TestHarness?) {
        val y = harness?.group ?: "fallback"
    }

    // Function call that may return null by convention.
    fun findOrFallback(items: List<String>) {
        val y = items.find { it.isEmpty() } ?: "fallback"
    }

    // Call-expression whose callee is a safe-call chain. The call's
    // declared return type is non-nullable Int, but the chain short-
    // circuits to null when `s` is null, so the elvis fallback is real.
    fun safeCallTerminatingInCall(s: String?) {
        val y = s?.length?.toInt() ?: 0
    }

    // Navigation chain that ends in `.bar` whose receiver is a safe-call
    // call_expression. The outer step is `.`, but the receiver propagates
    // null upstream — the elvis must be preserved.
    fun navigationAfterSafeCall(harness: TestHarness?) {
        val y = harness?.group?.length.toString() ?: "fallback"
    }

    // Member access on a receiver whose type is not declared in this file.
    // Source inference cannot see the member's declared nullability and must
    // not default it to non-null: the member may well be nullable, so the
    // elvis fallback is meaningful.
    fun memberAccessOnUnknownReceiver(decl: ExternalDecl) {
        val y = decl.source ?: return
        use(y)
    }

    // Member access to a name that does not exist on a known in-file class.
    // The receiver type resolves, but the member does not — its nullability
    // is unproven, so the rule must not fire.
    fun memberAccessUnknownMember(h: TestHarness) {
        val y = h.missing ?: "fallback"
        use(y)
    }

    // Qualified call whose target cannot be resolved in-file or in the stdlib.
    // The return type's nullability is unproven; the elvis must be preserved.
    fun qualifiedCallUnknownTarget(factory: ExternalFactory) {
        val y = factory.create() ?: return
        use(y)
    }

    fun unresolvedType(value: ExternalThing, fallback: ExternalThing): ExternalThing {
        return value ?: fallback
    }

    fun use(x: Any?) {}
}

class TestHarness {
    val group: String? = null
}

class NullableElvisBailProperties {
    class SomeType

    val logFile: SomeType? = null

    fun nullablePropertyBail() {
        val f = logFile ?: return
        use(f)
    }

    val p: SomeType?
        get() = null

    fun customGetterBail() {
        val value = p ?: return
        use(value)
    }

    fun use(value: Any?) {}
}

class ShadowedNullableElvisBail {
    val x: String? = null

    fun shadowingInitializer() {
        val x = x ?: return
        use(x)
    }

    fun use(value: Any?) {}
}

// Issue #1116: `Flow<T?>.first()` returns `T?`; after `.catch {}` the element type is unknown to source
// inference, which must not default it to a non-null `Any`.
class ElvisFlowChannel(val name: String)

interface ElvisFlowRepo {
    fun byId(id: String): Flow<ElvisFlowChannel?>
}

class ElvisFlowElementResolver(private val repo: ElvisFlowRepo) {
    suspend fun source(id: String): String {
        val channel =
            repo
                .byId(id)
                .catch { emit(null) }
                .first() ?: return "other"
        return channel.name
    }

    suspend fun direct(id: String): String {
        val channel = repo.byId(id).first() ?: return "other"
        return channel.name
    }
}

// Issue #1116: inside `fun T?.f()`, `this` is the nullable receiver, not the enclosing class.
class ElvisReceiverConfig(val name: String)

class ElvisReceiverCache {
    fun default(): ElvisReceiverConfig = ElvisReceiverConfig("d")
}

class ElvisNullableReceiverOverlay(private val cache: ElvisReceiverCache) {
    private fun ElvisReceiverConfig?.withLayout(): ElvisReceiverConfig {
        val config = this ?: cache.default()
        return config
    }

    private val ElvisReceiverConfig?.label: String
        get() = this?.name ?: "none"

    private val ElvisReceiverConfig?.orDefault: ElvisReceiverConfig
        get() = this ?: cache.default()
}

// Issue #1116: `a != null || b` does not prove `a != null`.
fun elvisDisjunctionGuard(navigationKey: String?, changed: Boolean): String {
    if (navigationKey != null || changed) {
        val key = navigationKey ?: "default"
        return key
    }
    if ((navigationKey != null) || changed) {
        return navigationKey ?: "default"
    }
    if (!(navigationKey == null)) {
        return ""
    }
    return navigationKey ?: ""
}

// Issue #1116: inside an extension, a bare name may be a member of the receiver, which shadows an outer
// declaration of the same name (okhttp's `val address = address ?: return hostName`).
class ElvisReceiverMemberHolder(val label: String?)

class ElvisReceiverMemberShadow(private val label: String, private val address: ElvisReceiverConfig) {
    fun ElvisReceiverMemberHolder.labelOrDefault(): String = label ?: "none"

    companion object {
        val java.net.InetSocketAddress.socketHost: String
            get() {
                val address = address ?: return hostName
                return address.hostAddress
            }
    }
}
