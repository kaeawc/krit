// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Fragment subclasses without a no-arg constructor that Go misses. Each is
// an Android Fragment the framework cannot re-instantiate, so each is
// reported; the comments name why Go misses it.
package test

import androidx.fragment.app.DialogFragment
import androidx.fragment.app.Fragment as AndroidFragment

abstract class BaseFragment : AndroidFragment()

open class ScreenFragment : BaseFragment()

// Go misses this because it reads only the direct supertype's name.
<!FragmentConstructor!>class<!> Home(val userId: String) : BaseFragment()

// Go misses this because it reads only the direct supertype's name.
<!FragmentConstructor!>class<!> Settings(val section: Int) : ScreenFragment()

open class BaseDialog : DialogFragment()

// Go misses this because it reads only the direct supertype's name.
<!FragmentConstructor!>class<!> Confirm(val message: String) : BaseDialog()

typealias Page = androidx.fragment.app.Fragment

// Go misses this because it does not expand the type alias.
<!FragmentConstructor!>class<!> Aliased(val index: Int) : Page()

// Go misses this because it does not follow the import alias.
<!FragmentConstructor!>class<!> ImportAliased(val index: Int) : AndroidFragment()

// Go misses this because, with no primary constructor, only a primary
// constructor with parameters makes it look for a no-arg one.
<!FragmentConstructor!>class<!> SecondaryOnly : AndroidFragment {
    constructor(id: Int) : super()
    constructor(id: Int, name: String) : super()
}

// Go misses this because, with no primary constructor, it never looks for a
// no-arg one. Without @JvmOverloads a defaulted secondary constructor has no
// no-arg JVM overload, so the framework's reflective instantiation fails even
// though Kotlin callers can write SecondaryDefaultOnly().
<!FragmentConstructor!>class<!> SecondaryDefaultOnly : AndroidFragment {
    constructor(a: Int = 0) : super()
}

// Go misses this because it takes the no-arg constructor of the nested class
// for the Fragment's own.
<!FragmentConstructor!>class<!> NestedMasks(val id: Int) : AndroidFragment() {
    class Holder {
        constructor()
    }
}

// Go misses this because it takes the no-arg constructor of the local class
// in a member function for the Fragment's own.
<!FragmentConstructor!>class<!> LocalMasks(val id: Int) : AndroidFragment() {
    fun helper(): Any {
        class Local {
            constructor()
        }
        return Local()
    }
}

fun localBase(): AndroidFragment {
    open class LocalBase : AndroidFragment()

    // Go misses this because it reads only the direct supertype's name.
    <!FragmentConstructor!>class<!> LocalChild(val id: Int) : LocalBase()
    return LocalChild(1)
}

// Classes declared inside an object expression resolve through the same
// supertype lookup without crashing.
private val holder = object {
    open inner class Base : AndroidFragment()

    // Go misses this because it reads only the direct supertype's name.
    <!FragmentConstructor!>inner<!> class Child(val a: Int) : Base()
}

// --- Negatives: the same shapes with a no-arg constructor ---

class HomeNoArg(val userId: String = "") : BaseFragment()

class AliasedNoArg : Page()

class SecondaryNoArgOnly : AndroidFragment {
    constructor() : super()
}

abstract class AbstractChild(val id: Int) : BaseFragment()
