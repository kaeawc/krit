package fixtures.positive.resourcecost

import androidx.work.PeriodicWorkRequestBuilder
import java.util.concurrent.TimeUnit

class PeriodicWorkRequestLessThan15Min {
    fun scheduleWork() {
        val request = PeriodicWorkRequestBuilder<MyWorker>(5, TimeUnit.MINUTES)
            .build()
    }
}
