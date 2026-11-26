package com.example.synth

import android.content.Context
import androidx.work.Worker
import androidx.work.WorkerParameters

private class UploadWorker(context: Context, params: WorkerParameters) : Worker(context, params) {
    override fun doWork() = Result.success()
}
