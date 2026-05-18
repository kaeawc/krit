package exceptions

// Bare TODO() and kotlin.TODO() are the only forms that resolve to
// kotlin.TODO; navigation calls on user types must not be flagged just
// because the rightmost identifier happens to be TODO.

class Service {
    fun TODO(): String = "user-defined TODO"
    fun other(): String = "x"
}

class Caller(private val svc: Service) {
    fun a(): String = svc.TODO()
    fun b(): String = svc.other()
}

object Registry {
    fun TODO(): String = "object-defined TODO"
}

fun useObjectMember(): String = Registry.TODO()

class Nested {
    inner class Inner {
        fun TODO(): String = "inner TODO"
    }

    fun useInner(): String {
        val inner = Inner()
        return inner.TODO()
    }

    fun useDeepChain(c: Caller): String = c.svc.TODO()
}

fun ordinaryWork() {
    doWork()
}

private fun doWork() {
    println("working")
}

// Local lookalikes: member functions named `TODO` on non-kotlin receivers must
// not be flagged as kotlin.TODO() calls.
class Tracker {
    fun TODO(label: String) {
        println("tracking $label")
    }
}

object Items {
    fun TODO() {
        println("queued")
    }
}

class Caller {
    private val tracker = Tracker()
    private val nested = Nested()

    fun useReceivers() {
        // foo.TODO() must not be treated as kotlin.TODO().
        tracker.TODO("future")
        Items.TODO()
        nested.api.TODO()
    }

    class Nested {
        val api = Tracker()
    }
}
