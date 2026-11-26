package fixtures.fixable

import android.content.Context
import androidx.work.Data
import androidx.work.RxWorker
import androidx.work.WorkerParameters
import io.reactivex.Single

class UploadWorker(context: Context, params: WorkerParameters) : RxWorker(context, params) {
    override fun createWork(): Single<Result> {
        setProgress(Data.EMPTY)
        val pending = setProgress(Data.EMPTY)
        return pending.map { Result.success() }
    }
}
