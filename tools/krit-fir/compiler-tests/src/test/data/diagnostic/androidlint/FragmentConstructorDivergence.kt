// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 25
// @JvmOverloads secondary constructors. Go reports a Fragment subclass whose
// annotated, fully defaulted secondary constructor gives it a no-arg JVM
// constructor; that class is not reported here.
package test

import androidx.fragment.app.Fragment

// Go reports this because it never reads secondary constructor defaults or
// @JvmOverloads; the compiler emits a no-arg JVM overload for the annotated
// constructor, so the framework can re-instantiate the class.
class Overloaded(val name: String) : Fragment() {
    @JvmOverloads
    constructor(a: Int = 0, b: Int = 0) : this("${a + b}")
}

class OverloadedOnly : Fragment {
    // Go reports nothing here: with no primary constructor it never looks.
    @JvmOverloads
    constructor(a: Int = 0) : super()
}

// @JvmOverloads needs every parameter defaulted to produce a no-arg overload.
<!FragmentConstructor!>class<!> PartlyOverloaded(val name: String) : Fragment() {
    @JvmOverloads
    constructor(a: Int, b: Int = 0) : this("${a + b}")
}
