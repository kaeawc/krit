// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 27, 31, 33, 35, 37, 42, 47, 49, 53, 57, 61, 64, 68, 71, 75, 79, 83, 86
// Android components the framework cannot instantiate: the class is private,
// or its primary constructor is private. Go reports each of these too.
package test

import android.app.Activity
import android.app.Application
import android.app.IntentService
import android.app.Service
import android.content.BroadcastReceiver
import android.content.ContentProvider
import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.os.IBinder
import androidx.activity.ComponentActivity
import androidx.appcompat.app.AppCompatActivity
import androidx.fragment.app.FragmentActivity

<!Instantiatable!>class<!> PrivateCtorActivity private constructor() : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }
}

<!Instantiatable!>private<!> class PrivateService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
}

<!Instantiatable!>class<!> PrivateCtorAppCompat private constructor(val id: Int) : AppCompatActivity()

<!Instantiatable!>class<!> PrivateCtorComponent private constructor() : ComponentActivity()

<!Instantiatable!>internal<!> class PrivateCtorFragmentActivity private constructor() : FragmentActivity()

<!Instantiatable!>class<!> PrivateCtorIntentService private constructor() : IntentService("worker") {
    override fun onHandleIntent(intent: Intent?) {
    }
}

<!Instantiatable!>private<!> class PrivateReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context?, intent: Intent?) {
    }
}

<!Instantiatable!>private<!> abstract class PrivateProvider : ContentProvider()

<!Instantiatable!>class<!> PrivateCtorApplication private constructor() : Application()

// The modifier list starts the declaration, so the finding sits on the
// annotation's line, where Go reports class_declaration.
<!Instantiatable!>@Deprecated("legacy")<!>
open class AnnotatedActivity private constructor() : Activity()

/** KDoc is not part of the declaration's first line. */
<!Instantiatable!>class<!> DocumentedActivity private constructor() : Activity()

// A private primary constructor with only defaulted parameters still gives
// the framework no public no-arg constructor.
<!Instantiatable!>class<!> DefaultedActivity private constructor(val id: Int = 0) : Activity()

// A private secondary constructor is not a public no-arg constructor.
<!Instantiatable!>class<!> PrivateSecondaryActivity private constructor(val id: Int) : Activity() {
    private constructor() : this(0)
}

<!Instantiatable!>class<!> AnnotatedCtorActivity @Deprecated("x") private constructor() : Activity()

class Outer {
    <!Instantiatable!>private<!> class NestedService : Service() {
        override fun onBind(intent: Intent?): IBinder? = null
    }

    <!Instantiatable!>inner<!> class InnerActivity private constructor() : Activity()
}

fun host() {
    <!Instantiatable!>class<!> LocalActivity private constructor() : Activity()
}

// A qualified supertype.
<!Instantiatable!>class<!> QualifiedActivity private constructor() : android.app.Activity()

// Both the class and its constructor are private: one finding.
<!Instantiatable!>private<!> class DoublyPrivateActivity private constructor() : Activity()

// Negatives: instantiatable components.
class PublicActivity : Activity()

class PublicCtorActivity constructor() : Activity()

internal class InternalService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
}

open class ProtectedCtorActivity protected constructor() : Activity()

class InternalCtorActivity internal constructor() : Activity()

// Go checks only `private`; a public constructor with parameters is not
// reported by either.
class ParamActivity(val id: Int) : Activity()

// Negatives: not components.
class Helper private constructor()

private class PrivateHelper

private interface Listener

// Go never visits object declarations.
private object SingletonActivity : Activity()
