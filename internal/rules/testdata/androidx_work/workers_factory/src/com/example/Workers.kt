package com.example

import android.content.Context
import androidx.work.Configuration
import androidx.work.Worker
import androidx.work.WorkerFactory
import androidx.work.WorkerParameters

internal class InternalWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

private class PrivateWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

fun config(factory: WorkerFactory) = Configuration.Builder().setWorkerFactory(factory).build()
