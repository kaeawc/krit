package exceptions

class Example {
    fun foo() {
        doWork()
    }

    private fun doWork() {
        println("working")
    }
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
