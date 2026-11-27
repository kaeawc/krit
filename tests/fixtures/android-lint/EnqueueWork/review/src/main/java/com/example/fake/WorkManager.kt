package com.example.fake

class WorkManager {
    fun beginWith(x: Int): WorkManager = this

    fun then(x: Int): WorkManager = this

    companion object {
        fun getInstance(context: Any): WorkManager = WorkManager()
    }
}

// A local class named like the androidx one, without its import.
fun lookalikeManager(context: Any) {
    WorkManager.getInstance(context).beginWith(1)
}
