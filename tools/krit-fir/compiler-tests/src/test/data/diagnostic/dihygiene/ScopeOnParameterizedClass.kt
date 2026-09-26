// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 28, 31, 34, 37, 40, 43, 46, 51, 56, 60, 63, 66, 70, 73, 76, 79, 83, 87, 91, 95, 101, 106, 111
// Scoped generic classes, the shapes Go reports: the finding sits on the
// declaration's first line (its modifier list), like Go.
package test

import dagger.Reusable
import dagger.hilt.android.scopes.ActivityRetainedScoped
import dagger.hilt.android.scopes.ActivityScoped
import dagger.hilt.android.scopes.FragmentScoped
import dagger.hilt.android.scopes.ServiceScoped
import dagger.hilt.android.scopes.ViewModelScoped
import dagger.hilt.android.scopes.ViewScoped
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}

// Go misses this because the scope is written with its package; it is a
// scope on a generic class, so the message is true of it.
<!ScopeOnParameterizedClass!>@jakarta.inject.Singleton<!>
class JakartaCache<T> @jakarta.inject.Inject constructor()

<!ScopeOnParameterizedClass!>@Reusable<!>
class ReusableHolder<T> @Inject constructor()

<!ScopeOnParameterizedClass!>@ActivityScoped<!>
class ActivityStore<T> @Inject constructor()

<!ScopeOnParameterizedClass!>@ActivityRetainedScoped<!>
class RetainedStore<T> @Inject constructor()

<!ScopeOnParameterizedClass!>@FragmentScoped<!>
class FragmentStore<T> @Inject constructor()

<!ScopeOnParameterizedClass!>@ViewScoped<!>
class ViewStore<T> @Inject constructor()

<!ScopeOnParameterizedClass!>@ViewModelScoped<!>
class ViewModelStore<T> @Inject constructor()

<!ScopeOnParameterizedClass!>@ServiceScoped<!>
class ServiceStore<T> @Inject constructor()

// Two scopes: one finding, naming the scope that comes first in Go's list
// (Singleton), as Go does.
<!ScopeOnParameterizedClass!>@Reusable @Singleton<!>
class DoublyScoped<T>

// Other annotations and modifiers before the scope: the modifier list's first
// line.
<!ScopeOnParameterizedClass!>@Suppress("unused")<!>
@Singleton
internal class Stacked<T>

<!ScopeOnParameterizedClass!>@Named("cache") @Singleton<!> class Qualified<T>

/** KDoc is not part of the declaration's first line, in Go or here. */
<!ScopeOnParameterizedClass!>@Singleton<!>
class Documented<T>

<!ScopeOnParameterizedClass!>@Singleton<!>
class
    SplitHeader<T>

<!ScopeOnParameterizedClass!>@Singleton<!>
data class Pair2<A, B>(val a: A, val b: B)

<!ScopeOnParameterizedClass!>@Singleton<!>
abstract class AbstractRepo<T>

<!ScopeOnParameterizedClass!>@Singleton<!>
sealed class SealedRepo<T>

<!ScopeOnParameterizedClass!>@Singleton<!>
open class Bounded<T : CharSequence, R> where R : Comparable<R>

// Go reports interfaces too (tree-sitter parses them as class declarations).
<!ScopeOnParameterizedClass!>@Singleton<!>
interface Source<T>

class Outer<T> {
    <!ScopeOnParameterizedClass!>@Singleton<!>
    class Nested<R>

    // An inner class with its own type parameter.
    <!ScopeOnParameterizedClass!>@Singleton<!>
    inner class InnerGeneric<R>

    companion object {
        <!ScopeOnParameterizedClass!>@Singleton<!>
        class InCompanion<R>
    }
}

fun localScope() {
    <!ScopeOnParameterizedClass!>@Singleton<!>
    class Local<T>
}

val anonymous = object {
    <!ScopeOnParameterizedClass!>@Singleton<!>
    inner class InAnonymous<T>
}

// A backticked class name: the message quotes it with its backticks, like Go.
<!ScopeOnParameterizedClass!>@Singleton<!>
class `Back Tick`<T>

// --- Negatives ---

// Generic but unscoped.
class Unscoped<K, V> @Inject constructor()

// Scoped but not generic.
@Singleton
class UserRepository @Inject constructor()

@Singleton
object ScopedObject

// Only the outer class's type parameter: Go reads the class's own
// `type_parameters`, and so does this checker.
class Generic<T> {
    @Singleton
    inner class Inner
}

// The scope is on the constructor, not the class; Go reads only the class's
// own modifier list.
class ConstructorScoped<T> @Singleton @Inject constructor()

// A qualifier is not a scope.
@Named("cache")
class QualifiedOnly<T>

// A scope on a generic function or property is not a class.
@Singleton
fun <T> scopedFunction(): T? = null

// A local class in a generic function: the function's type parameter is not
// the class's own, so the class is not generic, in Go or here.
fun <T> genericFunction(value: T): Any {
    @Singleton
    class LocalInGeneric {
        val held: Any? = value
    }
    return LocalInGeneric()
}
