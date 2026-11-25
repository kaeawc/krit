package com.example.synth

import android.content.Context
import androidx.work.Worker
import androidx.work.WorkerParameters

class UploadWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

internal class InternalWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}

private abstract class BaseWorker(context: Context, params: WorkerParameters) : Worker(context, params)

private class Uploader(val context: Context)
