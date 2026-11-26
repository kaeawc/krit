package fixtures.negative.androidlint

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.RxWorker
import androidx.work.WorkerParameters
import io.reactivex.Single

class UploadWorker(context: Context, params: WorkerParameters) : RxWorker(context, params) {
    override fun createWork(): Single<Result> {
        setCompletableProgress(Data.EMPTY)
        return Single.just(Result.success())
    }
}

class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        setProgress(Data.EMPTY)
        return Result.success()
    }
}

class ProgressHolder {
    fun setProgress(data: Data) = Unit
    fun run() = setProgress(Data.EMPTY)
}
