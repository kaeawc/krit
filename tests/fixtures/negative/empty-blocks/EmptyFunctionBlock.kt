package fixtures.negative.emptyblocks

interface Listener {
    fun onEvent()
}

// (1) Function with a real body — never flagged.
fun doSomething() {
    println("hi")
}

// (2) override whose body documents the no-op.
class Impl : Listener {
    override fun onEvent() {
        // intentionally empty
    }
}

// (3) body whose only content is a line comment documenting the no-op.
fun bar() {
    // intentionally no-op
}

// (4) body whose only content is a block comment.
fun baz() {
    /* intentionally no-op */
}
