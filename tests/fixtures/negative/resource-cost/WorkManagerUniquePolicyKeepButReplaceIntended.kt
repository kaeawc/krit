package fixtures.negative.resourcecost

import androidx.work.ExistingWorkPolicy
import androidx.work.WorkManager

class WorkManagerUniquePolicyKeepButReplaceIntended {
    fun restartSync(workManager: WorkManager) {
        workManager.enqueueUniqueWork("sync", ExistingWorkPolicy.REPLACE, syncRequest)
    }
}
