package com.example

import android.util.Log

class MyActivity {
    companion object {
        const val TAG = "MyActivity"
    }

    fun doWork() {
        Log.d(TAG, "message")
    }
}

// Bug 1 fix: TAG in a nested class should be compared to the nested class name,
// not the outer class name.
class InstrumentationApplicationDependencyProvider {
    fun setup() {}

    class MockWebSocket {
        companion object {
            const val TAG = "MockWebSocket"
        }

        fun connect() {
            Log.d(TAG, "connecting")
        }
    }
}

// Bug 2 fix: Intentional prefix truncation for Android's 23-char log tag limit.
class BiometricDeviceAuthentication {
    companion object {
        const val TAG = "BiometricDeviceAuth"
    }

    fun authenticate() {
        Log.d(TAG, "authenticating")
    }
}

// Idiomatic form from the issue: ClassName::class.java.simpleName matching the
// enclosing class is treated as valid.
class PaymentFragment {
    companion object {
        private val TAG = PaymentFragment::class.java.simpleName
    }

    fun processPayment() {
        Log.d(TAG, "Processing payment")
    }
}
