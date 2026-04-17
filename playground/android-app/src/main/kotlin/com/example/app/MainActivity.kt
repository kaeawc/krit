package com.example.app

import android.os.Bundle
import android.widget.TextView
import android.widget.Toast
import androidx.annotation.WorkerThread
import androidx.appcompat.app.AppCompatActivity
import java.text.SimpleDateFormat
import java.util.Date

class MainActivity : AppCompatActivity() {

    private var textView: TextView? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        textView = findViewById(R.id.text_main)

        // ShowToast: makeText without .show()
        Toast.makeText(this, "Welcome!", Toast.LENGTH_SHORT)

        // SimpleDateFormat without Locale
        val dateFormat = SimpleDateFormat("yyyy-MM-dd")
        textView?.text = dateFormat.format(Date())

        loadData()
    }

    // WrongThread: worker-annotated method updates UI
    @WorkerThread
    fun loadData() {
        val data = fetchFromNetwork()
        textView?.text = data
    }

    private fun fetchFromNetwork(): String {
        Thread.sleep(1000)
        return "Hello from network"
    }

    // MagicNumber
    fun calculateRetryDelay(attempt: Int): Long {
        return attempt * 2000L + 500
    }

    // EmptyFunctionBlock
    fun onDataLoaded() {
    }

    // UnusedParameter: 'verbose' is never read
    fun formatTimestamp(millis: Long, verbose: Boolean): String {
        val sdf = SimpleDateFormat("HH:mm:ss")
        return sdf.format(Date(millis))
    }

    override fun onDestroy() {
        super.onDestroy()
        textView = null
    }
}
