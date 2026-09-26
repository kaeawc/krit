// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: every MutableSharedFlow call passes an argument, or the call is
// not inside a val/var declaration.
package test

import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.flow.MutableSharedFlow

class EventBus(
    // A constructor parameter's default value is not a property declaration.
    val injected: MutableSharedFlow<Int> = MutableSharedFlow(),
) {
    val replayed = MutableSharedFlow<String>(replay = 1)

    val buffered = MutableSharedFlow<String>(extraBufferCapacity = 64)

    val positional = MutableSharedFlow<String>(1)

    val explicitZero = MutableSharedFlow<String>(replay = 0, extraBufferCapacity = 0)

    val overflowOnly = MutableSharedFlow<String>(onBufferOverflow = BufferOverflow.SUSPEND)

    val untyped: MutableSharedFlow<String> = MutableSharedFlow(replay = 1)

    val multiline = MutableSharedFlow<String>(
        replay = 1,
    )

    // A reference is not a call.
    val factory: () -> MutableSharedFlow<Int> = ::MutableSharedFlow

    fun create() = MutableSharedFlow<Int>()

    fun block(): MutableSharedFlow<Int> {
        return MutableSharedFlow()
    }

    fun subject(): Int = when (val flow = MutableSharedFlow<Int>()) {
        else -> flow.subscriptionCount.value
    }

    fun loop() {
        for (flow in listOf(MutableSharedFlow<Int>())) {
            flow.tryEmit(1)
        }
    }
}
