// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 26, 33, 42, 51
// Fragment subclasses Go reports that do have a no-arg constructor: an
// annotated, fully defaulted @JvmOverloads secondary constructor, and a
// no-arg secondary constructor whose body holds a local function, a function
// type or an anonymous function. None of them is reported here.
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

// Go reports this because it counts the local function's parameter as the
// constructor's; the class does have a no-arg constructor.
class LocalFunInCtor(val a: Int) : Fragment() {
    constructor() : this(0) {
        fun helper(x: Int) = x
        helper(1)
    }
}

// Go reports this because it counts the function type's named parameter as
// the constructor's; the class does have a no-arg constructor.
class FunctionTypeInCtor(val a: Int) : Fragment() {
    constructor() : this(0) {
        val f: (x: Int) -> Unit = {}
        f(1)
    }
}

// Go reports this because it counts the anonymous function's parameter as
// the constructor's; the class does have a no-arg constructor.
class AnonymousFunInCtor(val a: Int) : Fragment() {
    constructor() : this(0) {
        val g = fun(y: Int) = y
        g(1)
    }
}
