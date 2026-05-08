package complexity

class LargeClass {
    val x1 = 1
    val x2 = 2
    val x3 = 3
    val x4 = 4
    val x5 = 5

    fun doSomething(): Int {
        return x1 + x2 + x3
    }

    fun doSomethingElse(): Int {
        return x4 + x5
    }

    companion object {
        const val TAG = "LargeClass"
    }
}
