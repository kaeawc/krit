package com.example

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.RxWorker
import androidx.work.WorkerParameters
import io.reactivex.Single

class Rx2Worker(context: Context, params: WorkerParameters) : RxWorker(context, params) {
    override fun createWork(): Single<Result> {
        setProgress(Data.EMPTY)
        this.setProgress(Data.EMPTY)
        setCompletableProgress(Data.EMPTY)
        return Single.just(Result.success())
    }
}

abstract class Rx2Base(context: Context, params: WorkerParameters) : RxWorker(context, params)

class Rx2Child(context: Context, params: WorkerParameters) : Rx2Base(context, params) {
    override fun createWork(): Single<Result> {
        setProgress(Data.EMPTY)
        return Single.just(Result.success())
    }
}

fun external(worker: Rx2Worker) {
    worker.setProgress(Data.EMPTY)
}

class Rx3Worker(context: Context, params: WorkerParameters) : androidx.work.rxjava3.RxWorker(context, params) {
    override fun createWork(): io.reactivex.rxjava3.core.Single<Result> {
        setProgress(Data.EMPTY)
        return io.reactivex.rxjava3.core.Single.just(Result.success())
    }
}

class CoWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        setProgress(Data.EMPTY)
        return Result.success()
    }
}

class Plain {
    fun setProgress(data: Data) = Unit
    fun run() {
        setProgress(Data.EMPTY)
    }
}
