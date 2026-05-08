package test

import android.app.Activity
import android.content.ComponentName
import android.content.Intent

class Screen : Activity() {
    fun guarded(uri: String, param: Intent) {
        val intent = Intent.parseUri(uri, 0)
        intent.setPackage(packageName)
        startActivity(intent)
        val intent2 = Intent()
        startActivity(intent2)
        startActivity(param)
    }

    fun componentGuard(uri: String) {
        val intent = Intent.parseUri(uri, 0)
        intent.component = ComponentName(packageName, "Target")
        startActivityForResult(intent, 7)
    }
}
