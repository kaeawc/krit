// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 26, 30
// Lookalike Handler classes in a file that also imports android.os.Handler.
package test

import android.os.Handler

object Foo {
    open class Handler
}

class Shadow {
    open class Handler

    // Go reports this: it resolves the name Handler through the explicit
    // import, but the nested class Shadow.Handler shadows the import, so this
    // is not android.os.Handler.
    inner class NestedShadow : Handler()
}

class Screen {
    <!HandlerLeak!>inner<!> class Genuine : Handler()

    // Go reports this: it resolves only the last segment `Handler` through the
    // explicit import, but Foo.Handler is not android.os.Handler.
    inner class Qualified : Foo.Handler()

    // Go reports this for the same reason; it is not an anonymous
    // android.os.Handler.
    val qualified = object : Foo.Handler() {}
}
