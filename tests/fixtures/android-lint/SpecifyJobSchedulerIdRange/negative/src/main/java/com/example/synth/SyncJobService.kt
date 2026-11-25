package com.example.synth

import android.app.job.JobParameters
import android.app.job.JobService
import androidx.work.Configuration

class SyncJobService : JobService() {
    override fun onStartJob(params: JobParameters?) = false
    override fun onStopJob(params: JobParameters?) = false
}

fun workConfiguration(): Configuration =
    Configuration.Builder().setJobSchedulerJobIdRange(10_000, 20_000).build()
