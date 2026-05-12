package test

import androidx.annotation.RequiresApi

@RequiresApi(26)
fun newApiHelper() {
}

@RequiresApi(value = 30)
fun otherHelper() {
}

class Caller {
    fun doWork() {
        newApiHelper()
        otherHelper()
    }
}
