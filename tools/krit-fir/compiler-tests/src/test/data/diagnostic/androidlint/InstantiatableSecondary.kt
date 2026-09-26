// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 21, 29, 37, 41, 45
// A private primary constructor next to a secondary constructor the
// framework can call with no arguments. These classes can be instantiated,
// so the message ("cannot be instantiated") is not true of them and FIR
// reports none. Go reports each, because it reads only the primary
// constructor's modifiers.
package test

import android.app.Activity
import android.app.Service
import android.content.Intent
import android.os.IBinder

// Go reports this: the public secondary constructor takes no arguments.
class SecondaryActivity private constructor(val id: Int) : Activity() {
    constructor() : this(0)
}

// Go reports this: an internal constructor is public in bytecode.
class InternalSecondaryService private constructor(val name: String) : Service() {
    internal constructor() : this("worker")

    override fun onBind(intent: Intent?): IBinder? = null
}

// Go reports this: @JvmOverloads gives the secondary constructor a no-arg
// overload.
class OverloadsActivity private constructor(val id: Int, val tag: String) : Activity() {
    @JvmOverloads
    constructor(tag: String = "main") : this(0, tag)
}

// Still reported, as by Go: a secondary constructor with a required
// parameter, a defaulted one without @JvmOverloads, and a protected no-arg
// one give the framework no public no-arg constructor.
<!Instantiatable!>class<!> RequiredSecondaryActivity private constructor() : Activity() {
    constructor(id: Int) : this()
}

<!Instantiatable!>class<!> DefaultedSecondaryActivity private constructor(val id: Int, val tag: String) : Activity() {
    constructor(tag: String = "main") : this(0, tag)
}

<!Instantiatable!>open<!> class ProtectedSecondaryActivity private constructor(val id: Int) : Activity() {
    protected constructor() : this(0)
}
