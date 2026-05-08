package fixtures.positive.resourcecost

import androidx.work.ExistingWorkPolicy
import androidx.work.WorkManager

class WorkManagerUniquePolicyKeepButReplaceIntended {
    fun restartSync(workManager: WorkManager) {
        workManager.cancelUniqueWork("sync")
        workManager.enqueueUniqueWork("sync", ExistingWorkPolicy.KEEP, syncRequest)
    }
}
