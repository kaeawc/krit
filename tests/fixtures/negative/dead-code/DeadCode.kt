package test

class MyClass {
    private fun helper(): String {
        return "used below"
    }

    fun publicMethod() {
        println(helper())
    }
}
