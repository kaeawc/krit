package com.example;

import android.os.Build;
import android.webkit.WebView;

class JWeb {
    void clients(WebView webView) {
        if (Build.VERSION.SDK_INT >= 26) {
            webView.getWebViewClient();
        } else {
            webView.reload();
        }
        if (Build.VERSION.SDK_INT < 23) {
            return;
        } else {
            webView.createWebMessageChannel();
        }
    }
}
