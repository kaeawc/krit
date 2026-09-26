// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 31, 33, 36, 38, 40, 43, 45, 47
// androidx.collection's ObjectList and ScatterSet declare their own
// firstOrNull / lastOrNull and any members (collection-jvm 1.5.0 signatures,
// declared here because no stub has them). Go reports the explicit-receiver
// comparisons; `any { }` exists, so `!= null` is a true positive the checker
// keeps. For `== null`, Go says 'Use '.none {}'', but these classes have only
// a predicate-less none(): the message is false of the code (and Go's fix,
// `list.none { ... }`, does not compile), so the checker drops it.
package androidx.collection

sealed class ObjectList<E> {
    fun none(): Boolean = TODO()
    fun any(): Boolean = TODO()
    fun any(predicate: (element: E) -> Boolean): Boolean = TODO()
    fun firstOrNull(): E? = TODO()
    fun firstOrNull(predicate: (element: E) -> Boolean): E? = TODO()
    fun lastOrNull(): E? = TODO()
    fun lastOrNull(predicate: (element: E) -> Boolean): E? = TODO()
}

class MutableObjectList<E> : ObjectList<E>()

sealed class ScatterSet<E> {
    fun any(): Boolean = TODO()
    fun none(): Boolean = TODO()
    fun firstOrNull(predicate: (element: E) -> Boolean): E? = TODO()
    fun any(predicate: (element: E) -> Boolean): Boolean = TODO()
}

fun objectListFirst(list: ObjectList<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.firstOrNull { it.isEmpty() } != null<!>

fun objectListLast(list: ObjectList<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.lastOrNull { it.isEmpty() } != null<!>

// An inherited member, with null on the left.
fun mutableObjectList(list: MutableObjectList<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>null != list.firstOrNull { it > 0 }<!>

fun scatterSetFirst(set: ScatterSet<String>): Boolean = <!UseAnyOrNoneInsteadOfFind!>set.firstOrNull { it.isEmpty() } != null<!>

fun safeCallObjectList(list: ObjectList<String>?): Boolean = <!UseAnyOrNoneInsteadOfFind!>list?.lastOrNull { it.isEmpty() } != null<!>

// Go reports these, but there is no none(predicate): the message is false.
fun objectListFirstIsNull(list: ObjectList<String>): Boolean = list.firstOrNull { it.isEmpty() } == null

fun objectListLastIsNull(list: ObjectList<String>): Boolean = list.lastOrNull { it.isEmpty() } == null

fun scatterSetIsNull(set: ScatterSet<String>): Boolean = set.firstOrNull { it.isEmpty() } == null

// No predicate: Go and the checker both leave it alone.
fun noPredicate(list: ObjectList<String>): Boolean = list.firstOrNull() != null
