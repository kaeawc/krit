package fixtures.positive.potentialbugs

import android.app.Activity

class MissingSuperCall : Activity() {
    override fun onCreate() {
        doWork()
    }

    private fun doWork() {
        println("work")
    }
}
