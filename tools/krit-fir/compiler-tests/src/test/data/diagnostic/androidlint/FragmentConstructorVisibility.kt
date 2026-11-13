// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 16, 18, 20, 27, 32, 35, 37
// Fragments the framework cannot re-instantiate for a reason other than a missing
// no-arg constructor, as in Android Lint 32.4.1 (ValidFragment): a private or
// protected class, an inner class, a private or protected no-arg constructor,
// and an anonymous object. Kotlin internal members compile to public JVM
// members, so they are accepted, which Lint is not.
package test

import androidx.fragment.app.Fragment
import javax.inject.Inject

<!FragmentConstructor!>private<!> class TopLevelPrivate : Fragment()

class Outer {
    <!FragmentConstructor!>private<!> class PrivateNested : Fragment()

    <!FragmentConstructor!>protected<!> class ProtectedNested : Fragment()

    <!FragmentConstructor!>inner<!> class InnerFragment : Fragment()

    class PlainNested : Fragment()

    internal class InternalNested : Fragment()
}

class PrivateNoArg <!FragmentConstructor!>private constructor()<!> : Fragment()

class InternalNoArg internal constructor() : Fragment()

class ProtectedSecondary : Fragment {
    <!FragmentConstructor!>protected constructor()<!> : super()
}

class PrivateWithArgument <!FragmentConstructor!>private<!> constructor(val a: Int) : Fragment()

val anonymous = <!FragmentConstructor!>object : Fragment() {}<!>

// A DI-annotated constructor means a custom FragmentFactory builds the class.
class Outer2 {
    private class Injected @Inject constructor(val a: Int) : Fragment()
}
