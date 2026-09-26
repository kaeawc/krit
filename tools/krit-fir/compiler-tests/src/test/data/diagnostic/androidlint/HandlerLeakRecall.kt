// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 27
// Handlers reached other than through a simple `Handler` name. Go resolves
// the import alias, but misses the fully qualified supertype, the type alias
// and the subclass of a Handler base class, because it resolves only the
// supertype's last name segment through the file's imports. Each of those is
// an inner or anonymous Handler, so it is reported here.
package test

import android.os.Handler as AndroidHandler

typealias PlatformHandler = android.os.Handler

open class BaseHandler : android.os.Handler()

class Screen {
    <!HandlerLeak!>inner<!> class Qualified : android.os.Handler()

    <!HandlerLeak!>inner<!> class Aliased : AndroidHandler()

    <!HandlerLeak!>inner<!> class TypeAliased : PlatformHandler()

    <!HandlerLeak!>inner<!> class Derived : BaseHandler()

    val qualified = <!HandlerLeak!>object<!> : android.os.Handler() {}

    val aliased = <!HandlerLeak!>object<!> : AndroidHandler() {}

    val derived = <!HandlerLeak!>object<!> : BaseHandler() {}
}
