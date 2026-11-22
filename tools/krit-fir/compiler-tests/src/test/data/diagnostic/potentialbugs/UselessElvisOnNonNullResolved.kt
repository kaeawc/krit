// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 30, 33
package elvis.resolved

typealias ElvisText = String

class ElvisResolved(val text: String) {
    var mutable: String = "value"
    val custom: String get() = "value"

    fun proven(value: String?, items: List<String>, flag: Boolean): String {
        // Go skips mutable identifiers; the declared String type proves non-null despite mutation.
        val mutableResult = <!UselessElvisOnNonNull!>mutable ?: "dead"<!>
        // Go skips member-initialized identifiers; FIR resolves their declared non-null result.
        val fromMember = this.text
        val memberResult = <!UselessElvisOnNonNull!>fromMember ?: "dead"<!>
        // Go misses this getter-backed initializer; the getter's signature is non-null String.
        val fromGetter = custom
        val getterResult = <!UselessElvisOnNonNull!>fromGetter ?: "dead"<!>
        // Go skips indexing; List<String>.get returns String, unlike nullable Map.get.
        val indexed = <!UselessElvisOnNonNull!>items[0] ?: "dead"<!>
        // Go skips if/when/block-shaped operands; both branches resolve to String.
        val branch = <!UselessElvisOnNonNull!>(if (flag) "yes" else "no") ?: "dead"<!>
        val selected = <!UselessElvisOnNonNull!>(when (flag) { true -> "yes"; false -> "no" }) ?: "dead"<!>
        // Both engines resolve stdlib joinToString to non-null String.
        val fromCall = listOf("item").joinToString()
        val callResult = <!UselessElvisOnNonNull!>fromCall ?: "dead"<!>
        // Both engines resolve this non-null type alias.
        val aliased: ElvisText = "value"
        val aliasResult = <!UselessElvisOnNonNull!>aliased ?: "dead"<!>
        // Both engines prove the parameter non-null after this early return.
        if (value == null) return "missing"
        val smart = <!UselessElvisOnNonNull!>value ?: "dead"<!>
        return mutableResult + memberResult + getterResult + indexed + branch + selected + callResult + aliasResult + smart
    }
}

// Go skips a generic type; a non-null upper bound is compiler proof.
fun <T : Any> elvisBounded(value: T, fallback: T): T = <!UselessElvisOnNonNull!>value ?: fallback<!>

// Go skips generic types; an intersection explicitly guarantees non-null.
fun <T> elvisDefinitely(value: T & Any, fallback: T & Any): T & Any = <!UselessElvisOnNonNull!>value ?: fallback<!>

// Go does not propagate the inner Elvis result; FIR resolves that result to String.
fun elvisNested(value: String?): String = <!UselessElvisOnNonNull!>(value ?: "inner") ?: "dead"<!>

// Go misses an asserted generic operand; !! produces T & Any even with a nullable bound.
fun <T> elvisAssertedGeneric(value: T, fallback: T & Any): T & Any = <!UselessElvisOnNonNull!>value!! ?: fallback<!>
