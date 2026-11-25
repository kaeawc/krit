// fir-parity: skip this fixture references undeclared Content, Wrapper and ExternalThing types, so it never compiles cleanly against the FIR stubs; Go remains authoritative for these cases and compiling FIR coverage is in compiler-tests data

package fixtures.negative.potentialbugs

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

class UnnecessarySafeCall {
    fun example(nullable: String?) {
        val len = nullable?.length
    }

    // Chained safe calls — upstream ?. makes receiver nullable
    fun chainedSafeCalls(content: Content?) {
        val seconds = content?.dataMessage?.expireTimer?.seconds
    }

    // Dotted member access — bare name resolution is unreliable
    fun dottedAccess(obj: Wrapper) {
        val value = obj.property?.name
    }

    // Nullable parameter with default — safe call is justified
    fun withNullableDefault(s: String? = null) {
        val len = s?.length
    }

    fun unresolvedType(value: ExternalThing) {
        println(value?.toString())
    }

    // String literals containing "this?." inside a lambda body must NOT
    // trip the repeated-`this?.` heuristic that suppresses findings on
    // `this?.X` in scope-function lambdas. The rule already does not flag
    // `this?.X` directly, but downstream callers consult the helper to
    // decide whether the receiver-this is nullable; an AST-based count
    // ensures only real navigation_expressions contribute.
    fun lambdaWithStringLiteralLookingLikeThisSafeCalls(obj: String?) {
        obj?.let {
            // Two textual occurrences of "this?." inside a single literal —
            // must not count as repeated AST navigations.
            val msg = "doc: this?.foo and this?.bar are not real calls"
            println(msg)
        }
    }
}

// A local shadows a same-named non-null declaration; only the nearest declaration in scope counts
// (#1116, #1117: `val session = users[...]` shadows the `session` property).
class SafeCallShadowUser(val name: String?)

class SafeCallShadowSession(val user: SafeCallShadowUser)

class SafeCallShadowPresenter(
    private val session: SafeCallShadowSession,
    private val users: Map<String, SafeCallShadowUser>,
) {
    fun firstName(): String? {
        val session = users[this.session.user.name ?: ""]
        return session?.name
    }
}

// ktor's OidcBearer: inside an extension, `parameters` is a receiver member, so the local `state` has an
// unknown type and must not borrow the non-null type of the outer `state` context parameter.
class SafeCallShadowState(val id: String)

class SafeCallShadowProvider(val parameters: Map<String, String>)

context(state: SafeCallShadowState)
fun SafeCallShadowProvider.settings(): () -> Int? = {
    val state = parameters["state"]
    state?.length
}

// Issue #1117: a destructured lambda parameter shadows an outer local of the same name.
data class SafeCallTranscriptState(val items: List<String>? = null)

class SafeCallDestructuringPresenter(private val states: Flow<SafeCallTranscriptState>) {
    fun present(): Flow<List<String>> {
        val items: List<String> = emptyList()
        return states.map { (items) ->
            items?.reversed() ?: emptyList()
        }
    }

    fun plainParameter(all: List<SafeCallTranscriptState>): List<Int?> {
        val items: List<String> = emptyList()
        return all.map { state -> state.items }.map { items -> items?.size }
    }
}

// Issue #1117: `this` inside `with(x)` / `x.run {}` / `x.apply {}` has x's type, so it is nullable when x is.
class SafeCallAccount(val plan: String)

class SafeCallAccountManager {
    fun find(id: String): SafeCallAccount? = null
}

class SafeCallWithHandler(private val manager: SafeCallAccountManager) {
    fun onChange(id: String) {
        with(manager.find(id)) {
            this?.let { println(it.plan) }
        }
    }

    fun onRun(account: SafeCallAccount?) {
        account.run { this?.plan }
        account.apply { this?.plan }
    }
}

// The implicit `it` of a lambda over nullable elements is nullable.
fun safeCallNullableElements(names: List<String?>) {
    names.forEach { println(it?.length) }
}
