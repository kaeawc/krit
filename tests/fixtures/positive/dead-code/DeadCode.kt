package test

class MyClass {
    private fun unusedHelper(): String {
        return "never called"
    }

    fun publicMethod() {
        println("hello")
    }
}
