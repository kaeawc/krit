package comments

class Example {
    /** Documented property. */
    val publicProp = 1
}

class Counter {
    /** Read-only view of a mutable list held in a Kotlin 2.4 explicit backing field. */
    val items: List<Int>
        field = mutableListOf()
}
