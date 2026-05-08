package test;

import android.content.pm.ApplicationInfo;
import android.webkit.WebView;

class App {
    void guardedByBuildConfig() {
        if (BuildConfig.DEBUG) {
            WebView.setWebContentsDebuggingEnabled(true);
        }
    }

    void guardedByApplicationInfo(ApplicationInfo applicationInfo) {
        if ((applicationInfo.flags & ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true);
        }
    }

    void argumentCarriesGuard() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG);
    }
}
