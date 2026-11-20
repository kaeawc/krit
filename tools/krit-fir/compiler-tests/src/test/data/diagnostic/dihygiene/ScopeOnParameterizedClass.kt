// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 50, 55, 59, 62, 65, 69, 72, 75, 78, 82, 86, 90, 94, 100, 105, 110
// Koin scoped generic classes retain declaration-first-line anchoring.
// Dagger 2.60.1 gives Cache<String> and Cache<Integer> separate instances.
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
import org.koin.core.annotation.Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}

// Inject scopes are not reported, including qualified references.
@jakarta.inject.Singleton
class JakartaCache<T> @jakarta.inject.Inject constructor()

@Reusable
class ReusableHolder<T> @Inject constructor()

@ActivityScoped
class ActivityStore<T> @Inject constructor()

@ActivityRetainedScoped
class RetainedStore<T> @Inject constructor()

@FragmentScoped
class FragmentStore<T> @Inject constructor()

@ViewScoped
class ViewStore<T> @Inject constructor()

@ViewModelScoped
class ViewModelStore<T> @Inject constructor()

@ServiceScoped
class ServiceStore<T> @Inject constructor()

// Only the Koin scope is supported; the finding anchors on the first modifier.
// Dagger Reusable does not itself trigger a finding.
<!ScopeOnParameterizedClass!>@Reusable<!> @Singleton
class DoublyScoped<T>

// Other annotations and modifiers before the scope: the modifier list's first
// line.
<!ScopeOnParameterizedClass!>@Suppress("unused")<!>
@Singleton
internal class Stacked<T>

<!ScopeOnParameterizedClass!>@Named("cache")<!> @Singleton class Qualified<T>

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
class ConstructorScoped<T> @javax.inject.Singleton @Inject constructor()

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

// The javax inject flavor is also silent.
@javax.inject.Singleton
class JavaxCache<T>
