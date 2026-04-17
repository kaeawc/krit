package com.example

import java.text.SimpleDateFormat
import java.util.Locale

class DateHelper {
    fun formatDate(): String {
        val sdf = SimpleDateFormat("yyyy-MM-dd", Locale.US)
        return sdf.format(java.util.Date())
    }
}
