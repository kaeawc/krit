// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 19, 24, 26, 28, 30, 33, 35, 37, 39, 41, 45, 47, 49, 52, 59, 65, 67, 71
// Fragment subclasses with a parameterized constructor and no no-arg
// constructor, the shapes Go reports: the finding sits on the declaration's
// first line (its modifier list, else `class`), like Go.
package test

import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject

<!FragmentConstructor!>class<!> PropertyParam(val itemId: Int) : Fragment()

<!FragmentConstructor!>class<!> PlainParam(userId: String) : Fragment() {
    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View? =
        inflater.inflate(android.R.layout.simple_list_item_1, container, false)
}

<!FragmentConstructor!>class<!> MixedDefaults(val a: Int, val b: String = "") : Fragment()

<!FragmentConstructor!>class<!> LayoutConstructor(val a: Int) : Fragment(android.R.layout.simple_list_item_1)

<!FragmentConstructor!>class<!> Dialog(private val title: String) : DialogFragment()

<!FragmentConstructor!>class<!> Qualified(val a: Int) : androidx.fragment.app.Fragment()

// Java interop: the framework Fragment classes are Java stubs.
<!FragmentConstructor!>class<!> Framework(val a: Int) : android.app.Fragment()

<!FragmentConstructor!>class<!> FrameworkDialog(val a: Int) : android.app.DialogFragment()

<!FragmentConstructor!>open<!> class OpenFragment(val a: Int) : Fragment()

<!FragmentConstructor!>internal<!> class Internal(val a: Int) : Fragment()

<!FragmentConstructor!>@AndroidEntryPoint<!>
class Injected @Inject constructor(private val dep: String) : Fragment()

/** KDoc is not part of the declaration's first line, in Go or here. */
<!FragmentConstructor!>class<!> Documented(val a: Int) : Fragment()

<!FragmentConstructor!>class<!> PrivatePrimary private constructor(val a: Int) : Fragment()

<!FragmentConstructor!>class<!> VarargOnly(vararg val ids: Int) : Fragment()

// Secondary constructors with parameters do not add a no-arg one.
<!FragmentConstructor!>class<!> PrimaryAndSecondary(val a: Int) : Fragment() {
    constructor(a: Int, b: Int) : this(a + b)
}

// A secondary constructor whose parameters all have defaults is callable
// with no arguments from Kotlin, but the compiler emits no no-arg JVM
// constructor for it, so the framework cannot re-instantiate the class.
<!FragmentConstructor!>class<!> SecondaryDefaults(val a: Int) : Fragment() {
    constructor(a: Int, b: Int = 0) : this(a + b)
    constructor(s: String = "") : this(s.length)
}

class Outer {
    <!FragmentConstructor!>class<!> Nested(val a: Int) : Fragment()

    <!FragmentConstructor!>inner<!> class Inner(val a: Int) : Fragment()
}

fun local(): Fragment {
    <!FragmentConstructor!>class<!> Local(val a: Int) : Fragment()
    return Local(1)
}

// --- Negatives ---

class NoConstructor : Fragment()

class EmptyPrimary() : Fragment()

class AllDefaults(val a: Int = 0, val b: String = "") : Fragment()

class SecondaryNoArg(val a: Int) : Fragment() {
    constructor() : this(0)
}

class SecondariesWithNoArg : Fragment {
    constructor() : super()
    constructor(a: Int) : super()
}

// Constructor visibility is not considered, as in Go.
class PrivateNoArg private constructor() : Fragment()

abstract class AbstractFragment(val a: Int) : Fragment()

sealed class SealedFragment(val a: Int) : Fragment()

class NotAFragment(val a: Int)

class NotAFragmentEither(val a: Int) : Comparable<NotAFragmentEither> {
    override fun compareTo(other: NotAFragmentEither): Int = a - other.a
}

object SingletonFragment : Fragment()

val anonymous = object : Fragment() {}

interface Screen

class ImplementsScreen(val a: Int) : Screen
