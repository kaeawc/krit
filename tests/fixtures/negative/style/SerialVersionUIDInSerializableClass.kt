package fixtures.negative.style

import java.io.Serializable

class Foo : Serializable {
    companion object {
        const val serialVersionUID = 1L
    }

    fun doSomething() {
        println("Has serialVersionUID")
    }
}

// Enum classes are implicitly Serializable on JVM but use name-based
// serialization, so serialVersionUID is irrelevant.
enum class Status : Serializable {
    ACTIVE, INACTIVE, PENDING
}

enum class Priority {
    LOW, MEDIUM, HIGH
}
