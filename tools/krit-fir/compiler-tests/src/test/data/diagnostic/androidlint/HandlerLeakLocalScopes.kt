// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 20
// Inner Handler classes inside local classes and anonymous objects, and
// Handler bases declared locally.
package test

import android.os.Handler

fun host(): Any {
    open class LocalBase : Handler()

    class Local {
        <!HandlerLeak!>inner<!> class InLocal : Handler()

        // Go misses this: it does not follow the local base class to Handler.
        <!HandlerLeak!>inner<!> class ViaLocalBase : LocalBase()
    }

    val holder = object {
        <!HandlerLeak!>inner<!> class InObject : Handler()
    }

    return listOf(Local(), holder)
}
