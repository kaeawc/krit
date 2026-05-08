package test

import android.content.pm.ApplicationInfo
import android.webkit.WebView

class App {
    fun guardedByBuildConfig() {
        if (BuildConfig.DEBUG) {
            WebView.setWebContentsDebuggingEnabled(true)
        }
    }

    fun guardedByApplicationInfo(applicationInfo: ApplicationInfo) {
        if ((applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true)
        }
    }

    fun argumentCarriesGuard() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
    }
}
