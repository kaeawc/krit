// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 33, 35, 36, 46, 49, 56
package test

val label = "default"
fun fetch(): Any = Any()
open class Entity(val id: Any)
open class Base { val label: Any = Any() }

// FIR resolves the inherited Any member; Go leaves this identifier unresolved.
class Row(id: String) : Entity(id) { fun same(other: Any) = id === other }

// Go rejects top-level bindings that may actually refer to implicit receiver members.
class GetterRow : Base() { val same get() = label === fetch() }
fun make(candidate: Any) = object : Base() { fun same() = label === candidate }
class MemberNode { val label: Any = Any() }
fun MemberNode.same(candidate: Any) = label === candidate
val MemberNode.same: Boolean get() = label === fetch()

// The receiver is unrelated to this top-level binding, so FIR reports it while Go remains silent.
class UnrelatedNode
fun UnrelatedNode.sameWith(candidate: Any) = <!AvoidReferentialEquality!>label === candidate<!>
// Runnable has no label member; FIR reports the unrelated top-level binding while Go stays silent.
class Screen : Runnable { override fun run() {}; fun same(other: Any) = <!AvoidReferentialEquality!>label === other<!> }

// Both engines resolve the lambda's implicit receiver to MemberNode.label, so neither reports here.
fun sameWith(node: MemberNode, other: Any) = with(node) { label === other }

// Enum members shadow top-level declarations just like ordinary class members.
enum class Mode { A; val label: Any = Any(); fun same(other: Any) = label === other }

// Findings here depend on resolver-provided String types, not syntactic bindings.
class PropertyRow(val id: String) { fun same(other: Any) = <!AvoidReferentialEquality!>id === other<!> }
class InitializedRow(id: String) {
    val same = <!AvoidReferentialEquality!>id === fetch()<!>
    init { <!AvoidReferentialEquality!>id === fetch()<!> }
}

// An unresolved companion member does not inherit syntactic constructor-parameter typing.
class CompanionRow(id: String) {
    companion object : Entity("x") { val same = id === fetch() }
}

// Go preserves resolved types inside classes and lambda captures.
class Captures {
    fun same(a: String, b: String) = run { <!AvoidReferentialEquality!>a === b<!> }
}
val text = "x"
class TopLevelUse { fun same(other: Any) = <!AvoidReferentialEquality!>text === other<!> }
open class StringBase { val text: String = "" }
// FIR resolves the inherited String; Go rejects the ambiguous top-level binding.
class InheritedUse : StringBase() { fun same(other: Any) = <!AvoidReferentialEquality!>text === other<!> }

// Local parameter captures keep their resolved types even with a supertype.
class InheritedCaptures : Base() {
    fun same(a: String, b: String) = run { <!AvoidReferentialEquality!>a === b<!> }
}
