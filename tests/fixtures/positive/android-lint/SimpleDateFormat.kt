package com.example

import java.text.SimpleDateFormat

class DateHelper {
    fun formatDate(): String {
        val sdf = SimpleDateFormat("yyyy-MM-dd")
        return sdf.format(java.util.Date())
    }
}
