package fixtures.positive.potentialbugs

open class Base {
    open fun onCreate() {
        println("base")
    }
}

class MissingSuperCall : Base() {
    override fun onCreate() {
        doWork()
    }

    private fun doWork() {
        println("work")
    }
}
