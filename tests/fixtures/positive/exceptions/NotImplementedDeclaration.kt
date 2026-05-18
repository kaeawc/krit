package exceptions

class Example {
    fun foo() {
        TODO("implement later")
    }

    fun bar() {
        // Fully-qualified kotlin.TODO is still a kotlin.TODO usage.
        kotlin.TODO("not yet")
    }
}
