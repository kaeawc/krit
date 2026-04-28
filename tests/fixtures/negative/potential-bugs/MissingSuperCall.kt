package fixtures.negative.potentialbugs

import android.app.Activity

class MissingSuperCallActivity : Activity() {
    override fun onCreate() {
        super.onCreate()
        doWork()
    }

    private fun doWork() {
        println("work")
    }
}

open class Base {
    open fun onCreate() {
        println("base")
    }
}

class MissingSuperCallLocalLookalike : Base() {
    override fun onCreate() {
        doWork()
    }

    private fun doWork() {
        println("work")
    }
}

interface Logger {
    fun log(message: String)
}

class MissingSuperCallOrdinaryOverride : Logger {
    override fun log(message: String) {
        println(message)
    }
}
