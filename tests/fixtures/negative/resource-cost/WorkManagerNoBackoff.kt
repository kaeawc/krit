package fixtures.negative.resourcecost

import androidx.work.BackoffPolicy
import androidx.work.OneTimeWorkRequestBuilder
import java.util.concurrent.TimeUnit

class WorkManagerNoBackoff {
    fun scheduleRetryWork() {
        val request = OneTimeWorkRequestBuilder<RetryWorker>()
            .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 30, TimeUnit.SECONDS)
            .addTag("retry")
            .build()
    }
}
