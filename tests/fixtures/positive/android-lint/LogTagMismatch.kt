package com.example

import android.util.Log

class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }

    fun doWork() {
        Log.d(TAG, "message")
    }
}

// Copy-paste error from the issue: TAG literal references the original class.
class PaymentFragment {
    companion object {
        private const val TAG = "CheckoutActivity"
    }

    fun processPayment() {
        Log.d(TAG, "Processing payment")
    }
}

// Same copy-paste error using the idiomatic ClassName::class.java.simpleName form.
class PaymentReceiver {
    companion object {
        private val TAG = CheckoutActivity::class.java.simpleName
    }

    fun handle() {
        Log.d(TAG, "received")
    }
}
