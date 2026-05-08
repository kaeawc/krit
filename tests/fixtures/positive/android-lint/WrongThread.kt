package com.example

import androidx.annotation.WorkerThread

@WorkerThread
fun loadData() {
    val data = fetchFromNetwork()
    textView.setText(data)
}
