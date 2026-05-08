package test

import android.app.Activity
import android.content.Intent

class Screen : Activity() {
    fun launch(uri: String) {
        val intent = Intent.parseUri(uri, 0)
        startActivity(intent)
    }
}
