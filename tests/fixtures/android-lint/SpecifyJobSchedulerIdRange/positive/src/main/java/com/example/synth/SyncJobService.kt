package com.example.synth

import android.app.job.JobParameters
import android.app.job.JobService

class SyncJobService : JobService() {
    override fun onStartJob(params: JobParameters?) = false
    override fun onStopJob(params: JobParameters?) = false
}
