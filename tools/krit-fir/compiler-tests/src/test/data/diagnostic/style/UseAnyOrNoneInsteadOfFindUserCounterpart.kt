// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 36, 38, 40, 42, 44, 46, 49, 61, 68
// A find that is not the standard library's, on a receiver that does have the
// suggested any / none with a predicate: a member of the class, an inherited
// member, or the stdlib Iterable extension. Go reports the explicit-receiver
// comparisons, the message is true, and the checker keeps them.
package test

class MyBag {
    fun find(predicate: (Int) -> Boolean): Int? = null
    fun any(predicate: (Int) -> Boolean): Boolean = false
    fun none(predicate: (Int) -> Boolean): Boolean = true
}

class Node

class Tree : Iterable<Node> {
    override fun iterator(): Iterator<Node> = emptyList<Node>().iterator()
    fun find(predicate: (Node) -> Boolean): Node? = null
}

open class Base {
    fun any(predicate: (String) -> Boolean): Boolean = false
}

class Derived : Base() {
    fun lastOrNull(predicate: (String) -> Boolean): String? = null
}

// Only any: `== null` would need a none(predicate) the class does not have.
class AnyOnly {
    fun find(predicate: (Int) -> Boolean): Int? = null
    fun any(predicate: (Int) -> Boolean): Boolean = false
}

fun memberNotNull(bag: MyBag): Boolean = <!UseAnyOrNoneInsteadOfFind!>bag.find { it > 0 } != null<!>

fun memberIsNull(bag: MyBag): Boolean = <!UseAnyOrNoneInsteadOfFind!>bag.find { it > 0 } == null<!>

fun iterableSubclass(tree: Tree): Boolean = <!UseAnyOrNoneInsteadOfFind!>tree.find { it.hashCode() > 0 } != null<!>

fun iterableSubclassIsNull(tree: Tree): Boolean = <!UseAnyOrNoneInsteadOfFind!>tree.find { it.hashCode() > 0 } == null<!>

fun inheritedAny(derived: Derived): Boolean = <!UseAnyOrNoneInsteadOfFind!>derived.lastOrNull { it.isEmpty() } != null<!>

fun anyOnlyNotNull(bag: AnyOnly): Boolean = <!UseAnyOrNoneInsteadOfFind!>bag.find { it > 0 } != null<!>

// Go reports this, but AnyOnly has no none: the message is false.
fun anyOnlyIsNull(bag: AnyOnly): Boolean = bag.find { it > 0 } == null

// Go misses this: no explicit receiver. MyBag has any, so the checker reports.
fun implicitMember(bag: MyBag): Boolean = with(bag) { <!UseAnyOrNoneInsteadOfFind!>find { it > 0 } != null<!> }

// A local class and an object expression: the members are found through the
// receiver type's scope, never by resolving a class id.
fun localClass(): Boolean {
    class Local {
        fun find(predicate: (Int) -> Boolean): Int? = null
        fun any(predicate: (Int) -> Boolean): Boolean = false
    }
    return <!UseAnyOrNoneInsteadOfFind!>Local().find { it > 0 } != null<!>
}

val anonymousBag = object {
    fun find(predicate: (Int) -> Boolean): Int? = null
    fun none(predicate: (Int) -> Boolean): Boolean = true

    fun viaThis(): Boolean = <!UseAnyOrNoneInsteadOfFind!>this.find { it > 0 } == null<!>
}
