package fixtures.positive.resourcecost

import androidx.work.OneTimeWorkRequestBuilder

class WorkManagerNoBackoff {
    fun scheduleRetryWork() {
        val request = OneTimeWorkRequestBuilder<RetryWorker>()
            .addTag("retry")
            .build()
    }
}
