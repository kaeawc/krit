// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 16, 17, 18, 19, 20, 21, 22, 26, 30, 39, 46, 53, 60, 65
// Direct var bindings are shared state, regardless of their initializer's name.
package test

import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import kotlinx.coroutines.flow.MutableStateFlow as FlowState
import kotlin.properties.Delegates

object Counter {
    var <!MutableStateInObject!>total<!> = 0
    private var unused = 0
    var <!MutableStateInObject!>nullable<!>: String? = null
    lateinit var <!MutableStateInObject!>label<!>: String
    var <!MutableStateInObject!>flow<!> = FlowState(0)
    var <!MutableStateInObject!>compose<!> = mutableStateOf(0)
    var <!MutableStateInObject!>items<!> = mutableStateListOf(0)
    var <!MutableStateInObject!>delegated<!> by mutableStateOf(0)
    var <!MutableStateInObject!>observed<!> by Delegates.observable(0) { _, _, _ -> }

    /** Go anchors at the property name, not the KDoc or first modifier. */
    private
    var <!MutableStateInObject!>multiline<!> = 0
    fun writeMultiline() { multiline++ }

    object Nested {
        var <!MutableStateInObject!>nested<!> = 0
    }

    class Ordinary {
        var member = 0
    }
}

object PrivateHelper {
    private var <!MutableStateInObject!>count<!> = 0
    fun next(): Int = increment()
    private fun increment(): Int = ++count
}

interface Contract { var value: Int }
object Implementation : Contract {
    override var <!MutableStateInObject!>value<!> = 0
        get() = field
        set(value) { field = value }
}

object PublicSynchronized {
    // Public state is not exempt even if this object's own access is synchronized.
    var <!MutableStateInObject!>value<!> = 0
    @Synchronized fun next(): Int = ++value
}

fun mutableStateOf(value: String): String = value
object LocalFactory {
    // A project factory is not Compose state, but the var binding itself is unsafe in both rules.
    var <!MutableStateInObject!>binding<!> = test.mutableStateOf("local")
    val immutable = test.mutableStateOf("local")
}

object PrivateSetterWritten {
    var <!MutableStateInObject!>endpoint<!> = "x"
        private set
    fun update() { endpoint = "y" }
}

object InlineSynchronizedLambda {
    // Inline lambdas execute under the monitor, so the @Synchronized exemption remains valid.
    private var total = 0
    @Synchronized fun bump() { repeat(2) { total++ } }
}
