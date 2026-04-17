package test

import android.app.Activity

class MyActivity private constructor() : Activity() {
    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        super.onCreate(savedInstanceState)
    }
}
