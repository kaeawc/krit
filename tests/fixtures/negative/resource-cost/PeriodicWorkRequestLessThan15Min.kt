package fixtures.negative.resourcecost

import androidx.work.PeriodicWorkRequestBuilder
import java.util.concurrent.TimeUnit

class PeriodicWorkRequestLessThan15Min {
    fun scheduleWork() {
        val request = PeriodicWorkRequestBuilder<MyWorker>(15, TimeUnit.MINUTES)
            .build()
    }
}
