package com.example

import androidx.annotation.WorkerThread

@WorkerThread
fun loadData() {
    val data = fetchFromNetwork()
    runOnUiThread { textView.setText(data) }
}
