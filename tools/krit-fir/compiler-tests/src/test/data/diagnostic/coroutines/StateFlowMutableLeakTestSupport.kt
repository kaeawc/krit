// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 38
// Fakes, test helpers and @VisibleForTesting code expose a MutableStateFlow so
// tests can drive it, and a function-typed property holds one only inside its
// lambda, so none of these is reported. A plain public holder still is.
package test

import androidx.annotation.VisibleForTesting
import kotlinx.coroutines.flow.MutableStateFlow

class FakeRepository {
    val state = MutableStateFlow(0)
}

class TestHelper {
    val state = MutableStateFlow(1)
}

@VisibleForTesting
class ExposedForTests {
    val state = MutableStateFlow(3)
}

class FakeOuter {
    class Inner {
        val state = MutableStateFlow(4)
    }
}

class Holder {
    @VisibleForTesting
    val forTests = MutableStateFlow(5)

    val factory: () -> MutableStateFlow<Int> = { MutableStateFlow(6) }

    val withArgument: (Int) -> MutableStateFlow<Int> = { MutableStateFlow(it) }

    <!StateFlowMutableLeak!>val<!> leaked = MutableStateFlow(7)
}

/** Mutable by design: callers push state through it. */
interface Pusher {
    val state: MutableStateFlow<Int>
}

class MutableBox {
    val state = MutableStateFlow(8)
}

interface DocumentedMember {
    /** Mutable on purpose. */
    val state: MutableStateFlow<Int>
}
