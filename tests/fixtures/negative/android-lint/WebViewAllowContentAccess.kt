package com.example

import android.webkit.WebSettings
import android.webkit.WebView

class SettingsLookalike {
    var allowContentAccess: Boolean = false
}

class Page {
    fun bind(webView: WebView) {
        // Explicitly disabling is fine.
        webView.settings.allowContentAccess = false
        webView.settings.setAllowContentAccess(false)

        // Same property name on an unrelated receiver — must not flag.
        val s = SettingsLookalike()
        s.allowContentAccess = true
    }
}
