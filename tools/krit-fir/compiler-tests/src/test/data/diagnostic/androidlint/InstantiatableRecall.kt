// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// True positives Go misses. Go matches only a direct supertype whose written
// simple name is in its list (Activity, AppCompatActivity, Service, ...).
// Each class below is an Android component with a private class or primary
// constructor, so the framework cannot instantiate it.
package test

import android.app.Activity
import android.app.ListActivity
import android.app.Service as PlatformService
import android.content.Intent
import android.os.IBinder

abstract class BaseActivity : Activity()

// Go misses this because it reads only the direct supertype's name.
<!Instantiatable!>class<!> ChildActivity private constructor() : BaseActivity()

// Go misses this because ListActivity is not in its list.
<!Instantiatable!>private<!> class Entries : ListActivity()

typealias Screen = Activity

// Go misses this because it does not expand the type alias.
<!Instantiatable!>class<!> AliasedActivity private constructor() : Screen()

// Go misses this because it does not follow the import alias.
<!Instantiatable!>private<!> class AliasedService : PlatformService() {
    override fun onBind(intent: Intent?): IBinder? = null
}

// Go misses these because they have no primary constructor: every
// constructor is private.
<!Instantiatable!>class<!> SecondaryOnlyActivity : Activity {
    private constructor() : super()
}

<!Instantiatable!>class<!> TwoPrivateSecondaries : Activity {
    private constructor() : super()

    private constructor(id: Int) : this()
}

// Negatives: a public subclass of a component base class, a public no-arg
// secondary constructor next to a private one, and a public secondary
// constructor that takes arguments (neither Go nor FIR reports a public
// constructor with parameters).
class OpenChildActivity : BaseActivity()

class MixedSecondaries : Activity {
    private constructor(id: Int) : super()

    constructor() : this(0)
}

class ArgumentSecondary : Activity {
    constructor(id: Int) : super()
}
